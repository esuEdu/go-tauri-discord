use std::sync::Arc;
use std::time::Duration;

use rtc::interceptor::Registry;
use rtc::media::Sample;
use rtc::media_stream::MediaStreamTrack;
use rtc::peer_connection::configuration::interceptor_registry::register_default_interceptors;
use rtc::peer_connection::configuration::media_engine::{
    MediaEngine, MIME_TYPE_H264, MIME_TYPE_OPUS,
};
use rtc::peer_connection::configuration::RTCConfigurationBuilder;
use rtc::peer_connection::sdp::RTCSessionDescription;
use rtc::peer_connection::transport::RTCIceServer;
use rtc::rtp_transceiver::rtp_sender::{
    RTCRtpCodec, RTCRtpCodecParameters, RTCRtpCodingParameters, RTCRtpEncodingParameters,
    RtpCodecKind,
};
use rtc::rtp_transceiver::PayloadType;
use tokio::sync::mpsc::Receiver;
use tokio::sync::oneshot;
use webrtc::media_stream::track_local::static_sample::TrackLocalStaticSample;
use webrtc::media_stream::track_local::TrackLocal;
use webrtc::peer_connection::{
    PeerConnection, PeerConnectionBuilder, PeerConnectionEventHandler, RTCIceGatheringState,
    RTCPeerConnectionState,
};
use webrtc::runtime::default_runtime;

use crate::capture::Encoded;

const VIDEO_PAYLOAD: PayloadType = 102;
const AUDIO_PAYLOAD: PayloadType = 111;
const GATHER_TIMEOUT: Duration = Duration::from_secs(6);

const H264_FMTP: &str = "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f";

pub struct IceServer {
    pub urls: Vec<String>,
    pub username: Option<String>,
    pub credential: Option<String>,
}

struct Events {
    gathered: std::sync::Mutex<Option<oneshot::Sender<()>>>,
    ended: std::sync::Mutex<Option<oneshot::Sender<()>>>,
}

#[async_trait::async_trait]
impl PeerConnectionEventHandler for Events {
    async fn on_ice_gathering_state_change(&self, state: RTCIceGatheringState) {
        if state == RTCIceGatheringState::Complete {
            if let Ok(mut slot) = self.gathered.lock() {
                if let Some(tell) = slot.take() {
                    let _ = tell.send(());
                }
            }
        }
    }

    async fn on_connection_state_change(&self, state: RTCPeerConnectionState) {
        log::info!("screen: publish connection is {state}");
        if matches!(
            state,
            RTCPeerConnectionState::Failed
                | RTCPeerConnectionState::Closed
                | RTCPeerConnectionState::Disconnected
        ) {
            if let Ok(mut slot) = self.ended.lock() {
                if let Some(tell) = slot.take() {
                    let _ = tell.send(());
                }
            }
        }
    }
}

fn video_codec() -> RTCRtpCodecParameters {
    RTCRtpCodecParameters {
        rtp_codec: RTCRtpCodec {
            mime_type: MIME_TYPE_H264.to_owned(),
            clock_rate: 90_000,
            channels: 0,
            sdp_fmtp_line: H264_FMTP.to_owned(),
            rtcp_feedback: vec![],
        },
        payload_type: VIDEO_PAYLOAD,
    }
}

fn audio_codec() -> RTCRtpCodecParameters {
    RTCRtpCodecParameters {
        rtp_codec: RTCRtpCodec {
            mime_type: MIME_TYPE_OPUS.to_owned(),
            clock_rate: 48_000,
            channels: 2,
            sdp_fmtp_line: String::new(),
            rtcp_feedback: vec![],
        },
        payload_type: AUDIO_PAYLOAD,
    }
}

fn track_for(
    codec: &RTCRtpCodecParameters,
    kind: RtpCodecKind,
    stream_id: &str,
) -> Result<(Arc<TrackLocalStaticSample>, u32), String> {
    let ssrc = rand_ssrc();
    let track = TrackLocalStaticSample::new(MediaStreamTrack::new(
        stream_id.to_owned(),
        format!("vocalis-screen-{kind}"),
        format!("vocalis-screen-{kind}"),
        kind,
        vec![RTCRtpEncodingParameters {
            rtp_coding_parameters: RTCRtpCodingParameters {
                ssrc: Some(ssrc),
                ..Default::default()
            },
            codec: codec.rtp_codec.clone(),
            ..Default::default()
        }],
    ))
    .map_err(|error| format!("screen track: {error}"))?;

    Ok((Arc::new(track), ssrc))
}

fn rand_ssrc() -> u32 {
    use std::time::{SystemTime, UNIX_EPOCH};
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|since| since.subsec_nanos())
        .unwrap_or(1);
    (nanos | 1).wrapping_mul(2_654_435_761)
}

pub struct Publisher {
    pc: Arc<dyn PeerConnection>,
}

impl Publisher {
    pub async fn set_answer(&self, sdp: String) -> Result<(), String> {
        let answer = RTCSessionDescription::answer(sdp).map_err(|error| error.to_string())?;
        self.pc
            .set_remote_description(answer)
            .await
            .map_err(|error| error.to_string())
    }

    pub async fn add_candidate(
        &self,
        candidate: String,
        sdp_mid: Option<String>,
        sdp_mline_index: Option<u16>,
    ) -> Result<(), String> {
        self.pc
            .add_ice_candidate(rtc::peer_connection::transport::RTCIceCandidateInit {
                candidate,
                sdp_mid,
                sdp_mline_index,
                ..Default::default()
            })
            .await
            .map_err(|error| error.to_string())
    }

    pub async fn close(&self) {
        let _ = self.pc.close().await;
    }
}

pub struct Started {
    pub publisher: Publisher,
    pub offer: String,
    pub ended: oneshot::Receiver<()>,
}

pub async fn start(
    ice_servers: Vec<IceServer>,
    mut video: Receiver<Encoded>,
    mut audio: Option<Receiver<Encoded>>,
) -> Result<Started, String> {
    let mut media_engine = MediaEngine::default();
    media_engine
        .register_codec(video_codec(), RtpCodecKind::Video)
        .map_err(|error| error.to_string())?;
    if audio.is_some() {
        media_engine
            .register_codec(audio_codec(), RtpCodecKind::Audio)
            .map_err(|error| error.to_string())?;
    }

    let registry = register_default_interceptors(Registry::new(), &mut media_engine)
        .map_err(|error| error.to_string())?;

    let configuration = RTCConfigurationBuilder::new()
        .with_ice_servers(
            ice_servers
                .into_iter()
                .map(|server| RTCIceServer {
                    urls: server.urls,
                    username: server.username.unwrap_or_default(),
                    credential: server.credential.unwrap_or_default(),
                })
                .collect(),
        )
        .build();

    let (gathered_tx, gathered_rx) = oneshot::channel();
    let (ended_tx, ended_rx) = oneshot::channel();
    let events = Arc::new(Events {
        gathered: std::sync::Mutex::new(Some(gathered_tx)),
        ended: std::sync::Mutex::new(Some(ended_tx)),
    });

    let runtime = default_runtime().ok_or("no async runtime is compiled in")?;

    let pc = PeerConnectionBuilder::new()
        .with_configuration(configuration)
        .with_media_engine(media_engine)
        .with_interceptor_registry(registry)
        .with_handler(events)
        .with_runtime(runtime)
        .with_udp_addrs(vec!["0.0.0.0:0"])
        .build()
        .await
        .map_err(|error| format!("screen connection: {error}"))?;
    let pc: Arc<dyn PeerConnection> = Arc::new(pc);

    let stream_id = format!("vocalis-screen-{}", rand_ssrc());

    let (video_track, video_ssrc) = track_for(&video_codec(), RtpCodecKind::Video, &stream_id)?;
    pc.add_track(Arc::clone(&video_track) as Arc<dyn TrackLocal>)
        .await
        .map_err(|error| error.to_string())?;

    let audio_track = if audio.is_some() {
        let (track, ssrc) = track_for(&audio_codec(), RtpCodecKind::Audio, &stream_id)?;
        pc.add_track(Arc::clone(&track) as Arc<dyn TrackLocal>)
            .await
            .map_err(|error| error.to_string())?;
        Some((track, ssrc))
    } else {
        None
    };

    let offer = pc
        .create_offer(None)
        .await
        .map_err(|error| error.to_string())?;
    pc.set_local_description(offer)
        .await
        .map_err(|error| error.to_string())?;

    let _ = tokio::time::timeout(GATHER_TIMEOUT, gathered_rx).await;

    let offer = pc
        .local_description()
        .await
        .ok_or("the screen offer never became ready")?;

    tokio::spawn(async move {
        while let Some(frame) = video.recv().await {
            let sample = Sample {
                data: frame.data,
                duration: frame.duration,
                ..Default::default()
            };
            if video_track
                .sample_writer(video_ssrc, VIDEO_PAYLOAD)
                .write_sample(&sample)
                .await
                .is_err()
            {
                break;
            }
        }
    });

    if let (Some(mut audio), Some((track, ssrc))) = (audio.take(), audio_track) {
        tokio::spawn(async move {
            while let Some(chunk) = audio.recv().await {
                let sample = Sample {
                    data: chunk.data,
                    duration: chunk.duration,
                    ..Default::default()
                };
                if track
                    .sample_writer(ssrc, AUDIO_PAYLOAD)
                    .write_sample(&sample)
                    .await
                    .is_err()
                {
                    break;
                }
            }
        });
    }

    Ok(Started {
        publisher: Publisher { pc },
        offer: offer.sdp,
        ended: ended_rx,
    })
}
