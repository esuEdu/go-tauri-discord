use vocalis_lib::publish;

#[tokio::main]
async fn main() {
    let (_vt, video) = tokio::sync::mpsc::channel(8);
    let (_at, audio) = tokio::sync::mpsc::channel(8);

    let started = publish::start(
        vec![publish::IceServer {
            urls: vec!["stun:stun.l.google.com:19302".to_owned()],
            username: None,
            credential: None,
        }],
        video,
        Some(audio),
    )
    .await
    .expect("publisher starts");

    println!("{}", started.offer);
}
