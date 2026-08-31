use std::time::Duration;

use vocalis_lib::capture::{start, Options, Quality, Sink};
use vocalis_lib::sources;

#[tokio::main]
async fn main() {
    let found = sources::collect();
    println!("{} sources", found.len());
    for source in found.iter().take(8) {
        println!("  {} [{}] {}", source.id, source.kind, source.title);
    }

    let wanted = std::env::args().nth(1);
    let screen = match &wanted {
        Some(title) => found
            .iter()
            .find(|source| source.title.contains(title.as_str()))
            .expect("no source matched that title"),
        None => found
            .iter()
            .find(|source| source.kind == "screen")
            .expect("no screen to share"),
    };
    println!("capturing {} [{}]", screen.title, screen.kind);

    let target = sources::parse_target(&screen.id).expect("target parses");
    let (video_tx, mut video_rx) = tokio::sync::mpsc::channel(120);
    let (audio_tx, mut audio_rx) = tokio::sync::mpsc::channel(120);

    let session = start(
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

    let mut frames = 0usize;
    let mut bytes = 0usize;
    let mut keyframes = 0usize;
    let mut chunks = 0usize;

    let deadline = tokio::time::Instant::now() + Duration::from_secs(5);
    loop {
        tokio::select! {
            _ = tokio::time::sleep_until(deadline) => break,
            Some(frame) = video_rx.recv() => {
                frames += 1;
                bytes += frame.data.len();
                if frame.data.windows(5).any(|w| w[..4] == [0,0,0,1] && w[4] & 0x1F == 5) {
                    keyframes += 1;
                }
                if frames == 1 {
                    println!("first frame {} bytes, starts {:02x?}", frame.data.len(), &frame.data[..8.min(frame.data.len())]);
                }
            }
            Some(_) = audio_rx.recv() => { chunks += 1; }
        }
    }

    session.stop();

    println!("video: {frames} frames, {keyframes} keyframes, {bytes} bytes in 5s");
    println!("audio: {chunks} opus frames in 5s");

    if frames == 0 {
        eprintln!("NO VIDEO — is screen recording permission granted to this binary?");
        std::process::exit(1);
    }
}
