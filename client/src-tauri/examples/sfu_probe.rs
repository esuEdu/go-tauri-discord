use std::sync::Arc;
use std::time::Duration;

use futures_util::{SinkExt, StreamExt};
use serde_json::{json, Value};
use tokio::sync::mpsc::channel;
use tokio_tungstenite::tungstenite::Message;

use vocalis_lib::capture::{self, Options, Quality, Sink};
use vocalis_lib::publish::{self, IceServer};
use vocalis_lib::sources;

const OP_DISPATCH: i64 = 0;
const OP_HEARTBEAT: i64 = 1;
const OP_IDENTIFY: i64 = 2;
const OP_HELLO: i64 = 3;
const OP_VOICE_STATE: i64 = 7;
const OP_VOICE_OFFER: i64 = 8;
const OP_VOICE_ANSWER: i64 = 9;
const OP_VOICE_CANDIDATE: i64 = 10;
const OP_SCREEN_PUBLISH: i64 = 15;
const OP_SCREEN_ANSWER: i64 = 16;
const OP_SCREEN_ICE: i64 = 17;

#[tokio::main]
async fn main() {
    let token = std::env::var("VOCALIS_TOKEN").expect("VOCALIS_TOKEN");
    let channel_id = std::env::var("VOCALIS_CHANNEL").expect("VOCALIS_CHANNEL");
    let gateway = std::env::var("VOCALIS_GATEWAY")
        .unwrap_or_else(|_| "ws://localhost:8080/gateway".to_owned());

    let (socket, _) = tokio_tungstenite::connect_async(&gateway)
        .await
        .expect("gateway connects");
    let (mut write, mut read) = socket.split();
    println!("gateway connected");

    let (out_tx, mut out_rx) = channel::<Value>(64);
    tokio::spawn(async move {
        while let Some(frame) = out_rx.recv().await {
            if write
                .send(Message::Text(frame.to_string().into()))
                .await
                .is_err()
            {
                break;
            }
        }
    });

    let ice = Arc::new(tokio::sync::Mutex::new(Vec::<IceServer>::new()));
    let mut voice: Option<Arc<dyn webrtc::peer_connection::PeerConnection>> = None;
    let mut publisher: Option<publish::Publisher> = None;
    let mut in_room = false;
    let mut layers_sent = 0usize;
    let seen = Arc::new(std::sync::atomic::AtomicUsize::new(0));
    let watch_only = std::env::var_os("VOCALIS_WATCH_ONLY").is_some();

    let deadline = tokio::time::Instant::now() + Duration::from_secs(60);

    while let Ok(Some(Ok(message))) = tokio::time::timeout_at(deadline, read.next()).await {
        let Message::Text(text) = message else {
            continue;
        };
        let Ok(frame) = serde_json::from_str::<Value>(&text) else {
            continue;
        };
        let op = frame.get("op").and_then(Value::as_i64).unwrap_or(-1);
        let d = frame.get("d").cloned().unwrap_or(Value::Null);

        match op {
            OP_HELLO => {
                let beat = d
                    .get("heartbeat_interval_ms")
                    .and_then(Value::as_i64)
                    .unwrap_or(30_000);
                println!("hello, heartbeat {beat}ms");

                let tick = out_tx.clone();
                tokio::spawn(async move {
                    loop {
                        tokio::time::sleep(Duration::from_millis(beat as u64)).await;
                        if tick.send(json!({"op": OP_HEARTBEAT})).await.is_err() {
                            break;
                        }
                    }
                });

                out_tx
                    .send(json!({"op": OP_IDENTIFY, "d": {"token": token}}))
                    .await
                    .ok();
            }

            OP_DISPATCH => {
                let event = frame.get("t").and_then(Value::as_str).unwrap_or("");
                if event == "READY" {
                    let servers: Vec<IceServer> = d
                        .get("ice_servers")
                        .and_then(Value::as_array)
                        .map(|list| {
                            list.iter()
                                .map(|s| IceServer {
                                    urls: s
                                        .get("urls")
                                        .and_then(Value::as_array)
                                        .map(|u| {
                                            u.iter()
                                                .filter_map(|x| x.as_str().map(str::to_owned))
                                                .collect()
                                        })
                                        .unwrap_or_default(),
                                    username: s
                                        .get("username")
                                        .and_then(Value::as_str)
                                        .map(str::to_owned),
                                    credential: s
                                        .get("credential")
                                        .and_then(Value::as_str)
                                        .map(str::to_owned),
                                })
                                .collect()
                        })
                        .unwrap_or_default();
                    println!("ready, {} ice servers", servers.len());
                    *ice.lock().await = servers;

                    out_tx
                        .send(json!({"op": OP_VOICE_STATE, "d": {
                            "channel_id": channel_id,
                            "self_mute": false,
                            "self_deaf": false
                        }}))
                        .await
                        .ok();
                }
            }

            OP_VOICE_OFFER => {
                let sdp = d
                    .get("sdp")
                    .and_then(Value::as_str)
                    .unwrap_or("")
                    .to_owned();
                println!("voice offer, {} bytes", sdp.len());

                match voice.as_ref() {
                    Some(pc) => renegotiate(pc, &sdp, &out_tx).await,
                    None => voice = Some(answer_voice(&sdp, &out_tx, &ice, &seen).await),
                }
                in_room = true;
            }

            OP_VOICE_CANDIDATE => {
                if let Some(pc) = voice.as_ref() {
                    let init = rtc::peer_connection::transport::RTCIceCandidateInit {
                        candidate: d
                            .get("candidate")
                            .and_then(Value::as_str)
                            .unwrap_or("")
                            .to_owned(),
                        sdp_mid: d.get("sdp_mid").and_then(Value::as_str).map(str::to_owned),
                        sdp_mline_index: d
                            .get("sdp_mline_index")
                            .and_then(Value::as_u64)
                            .map(|i| i as u16),
                        ..Default::default()
                    };
                    let _ = pc.add_ice_candidate(init).await;
                }
            }

            OP_SCREEN_ANSWER => {
                let sdp = d.get("sdp").and_then(Value::as_str).unwrap_or("");
                println!("screen answer, {} bytes", sdp.len());
                if let Some(p) = publisher.as_ref() {
                    match p.set_answer(sdp.to_owned()).await {
                        Ok(()) => println!("screen answer applied"),
                        Err(error) => println!("screen answer rejected: {error}"),
                    }
                }
            }

            OP_SCREEN_ICE => {
                if let Some(p) = publisher.as_ref() {
                    let _ = p
                        .add_candidate(
                            d.get("candidate")
                                .and_then(Value::as_str)
                                .unwrap_or("")
                                .to_owned(),
                            d.get("sdp_mid").and_then(Value::as_str).map(str::to_owned),
                            d.get("sdp_mline_index")
                                .and_then(Value::as_u64)
                                .map(|i| i as u16),
                        )
                        .await;
                }
            }

            _ => {}
        }

        if watch_only {
            continue;
        }

        if in_room && publisher.is_none() {
            tokio::time::sleep(Duration::from_millis(600)).await;
            println!("publishing the screen");

            let found = sources::collect();
            let screen = found
                .iter()
                .find(|s| s.kind == "screen")
                .expect("a screen to share");
            let target = sources::parse_target(&screen.id).expect("target parses");

            let (video_tx, video_rx) = channel(120);
            let (audio_tx, audio_rx) = channel(120);

            let session = capture::start(
                Options {
                    target,
                    quality: Quality {
                        width: 1280,
                        height: 720,
                        frame_rate: 30,
                        max_bitrate: 3_000_000,
                    },
                    audio: true,
                },
                Sink {
                    video: video_tx,
                    audio: audio_tx,
                },
            )
            .expect("capture starts");
            std::mem::forget(session);

            let servers = std::mem::take(&mut *ice.lock().await);
            let started = publish::start(servers, video_rx, Some(audio_rx))
                .await
                .expect("publisher starts");

            out_tx
                .send(json!({"op": OP_SCREEN_PUBLISH, "d": {"sdp": started.offer}}))
                .await
                .ok();
            println!("offer sent, {} bytes", started.offer.len());

            publisher = Some(started.publisher);
            layers_sent += 1;
        }
    }

    let total = seen.load(std::sync::atomic::Ordering::Relaxed);
    println!(
        "done, published {layers_sent} time(s), received {} video and {} audio rtp payloads back",
        total / 1_000_000,
        total % 1_000_000
    );
}

async fn renegotiate(
    pc: &Arc<dyn webrtc::peer_connection::PeerConnection>,
    offer: &str,
    out: &tokio::sync::mpsc::Sender<Value>,
) {
    use rtc::peer_connection::sdp::RTCSessionDescription;

    let Ok(description) = RTCSessionDescription::offer(offer.to_owned()) else {
        return;
    };
    if pc.set_remote_description(description).await.is_err() {
        return;
    }
    let Ok(answer) = pc.create_answer(None).await else {
        return;
    };
    if pc.set_local_description(answer).await.is_err() {
        return;
    }
    tokio::time::sleep(Duration::from_millis(800)).await;
    if let Some(local) = pc.local_description().await {
        out.send(json!({"op": OP_VOICE_ANSWER, "d": {"type": "answer", "sdp": local.sdp}}))
            .await
            .ok();
        println!("renegotiated");
    }
}

async fn answer_voice(
    offer: &str,
    out: &tokio::sync::mpsc::Sender<Value>,
    ice: &Arc<tokio::sync::Mutex<Vec<IceServer>>>,
    seen: &Arc<std::sync::atomic::AtomicUsize>,
) -> Arc<dyn webrtc::peer_connection::PeerConnection> {
    use rtc::interceptor::Registry;
    use rtc::peer_connection::configuration::interceptor_registry::register_default_interceptors;
    use rtc::peer_connection::configuration::media_engine::MediaEngine;
    use rtc::peer_connection::configuration::RTCConfigurationBuilder;
    use rtc::peer_connection::sdp::RTCSessionDescription;
    use rtc::peer_connection::transport::RTCIceServer;
    use webrtc::peer_connection::{PeerConnection, PeerConnectionBuilder};

    struct Quiet {
        gathered: std::sync::Mutex<Option<tokio::sync::oneshot::Sender<()>>>,
        video: Arc<std::sync::atomic::AtomicUsize>,
    }

    #[async_trait::async_trait]
    impl webrtc::peer_connection::PeerConnectionEventHandler for Quiet {
        async fn on_connection_state_change(
            &self,
            state: webrtc::peer_connection::RTCPeerConnectionState,
        ) {
            println!("voice connection is {state}");
        }

        async fn on_ice_gathering_state_change(
            &self,
            state: webrtc::peer_connection::RTCIceGatheringState,
        ) {
            if state == webrtc::peer_connection::RTCIceGatheringState::Complete {
                if let Ok(mut slot) = self.gathered.lock() {
                    if let Some(tell) = slot.take() {
                        let _ = tell.send(());
                    }
                }
            }
        }

        async fn on_track(&self, track: Arc<dyn webrtc::media_stream::track_remote::TrackRemote>) {
            use webrtc::media_stream::track_remote::TrackRemoteEvent;
            use webrtc::media_stream::Track;

            let id = track.track_id().await;
            let kind = track.kind().await;
            let codec = match track.ssrcs().await.first() {
                Some(ssrc) => track
                    .codec(*ssrc)
                    .await
                    .map(|c| c.mime_type)
                    .unwrap_or_default(),
                None => String::new(),
            };
            println!("incoming track {id:?} {kind:?} {codec}");

            let seen = Arc::clone(&self.video);
            let is_video = matches!(kind, rtc::rtp_transceiver::rtp_sender::RtpCodecKind::Video);
            tokio::spawn(async move {
                while let Some(event) = track.poll().await {
                    if let TrackRemoteEvent::OnRtpPacket(packet) = event {
                        if !packet.payload.is_empty() {
                            let step = if is_video { 1_000_000 } else { 1 };
                            seen.fetch_add(step, std::sync::atomic::Ordering::Relaxed);
                        }
                    }
                }
            });
        }
    }

    let mut engine = MediaEngine::default();
    engine.register_default_codecs().expect("codecs");
    let registry = register_default_interceptors(Registry::new(), &mut engine).expect("registry");

    let servers: Vec<RTCIceServer> = ice
        .lock()
        .await
        .iter()
        .map(|s| RTCIceServer {
            urls: s.urls.clone(),
            username: s.username.clone().unwrap_or_default(),
            credential: s.credential.clone().unwrap_or_default(),
        })
        .collect();

    let (gathered_tx, gathered_rx) = tokio::sync::oneshot::channel();
    let pc = PeerConnectionBuilder::new()
        .with_configuration(
            RTCConfigurationBuilder::new()
                .with_ice_servers(servers)
                .build(),
        )
        .with_media_engine(engine)
        .with_interceptor_registry(registry)
        .with_handler(Arc::new(Quiet {
            gathered: std::sync::Mutex::new(Some(gathered_tx)),
            video: Arc::clone(seen),
        }))
        .with_runtime(webrtc::runtime::default_runtime().expect("runtime"))
        .with_udp_addrs(vec!["0.0.0.0:0"])
        .build()
        .await
        .expect("voice connection");
    let pc: Arc<dyn PeerConnection> = Arc::new(pc);

    pc.set_remote_description(RTCSessionDescription::offer(offer.to_owned()).expect("offer"))
        .await
        .expect("remote offer");
    let answer = pc.create_answer(None).await.expect("answer");
    pc.set_local_description(answer)
        .await
        .expect("local answer");

    let _ = tokio::time::timeout(Duration::from_secs(8), gathered_rx).await;
    let local = pc.local_description().await.expect("local description");

    out.send(json!({"op": OP_VOICE_ANSWER, "d": {"type": "answer", "sdp": local.sdp}}))
        .await
        .ok();
    println!("voice answered");

    pc
}
