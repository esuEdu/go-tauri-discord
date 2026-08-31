use super::{Options, Sink};

pub struct Session;

impl Session {
    pub fn stop(self) {}
}

pub fn start(_options: Options, _sink: Sink) -> Result<Session, String> {
    Err("native screen capture is not built on Windows yet".to_owned())
}
