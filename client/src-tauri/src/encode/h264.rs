use bytes::{BufMut, BytesMut};

pub const START_CODE: [u8; 4] = [0x00, 0x00, 0x00, 0x01];

pub fn append_annex_b(out: &mut BytesMut, nal: &[u8]) {
    out.put_slice(&START_CODE);
    out.put_slice(nal);
}

pub fn avcc_to_annex_b(avcc: &[u8], length_size: usize) -> Result<BytesMut, String> {
    if !(1..=4).contains(&length_size) {
        return Err(format!("unsupported nal length size {length_size}"));
    }

    let mut out = BytesMut::with_capacity(avcc.len() + 16);
    let mut at = 0usize;

    while at + length_size <= avcc.len() {
        let mut len = 0usize;
        for byte in &avcc[at..at + length_size] {
            len = (len << 8) | *byte as usize;
        }
        at += length_size;

        let end = at.checked_add(len).ok_or("nal length overflows")?;
        if end > avcc.len() {
            return Err("nal length runs past the buffer".to_owned());
        }
        append_annex_b(&mut out, &avcc[at..end]);
        at = end;
    }

    if at != avcc.len() {
        return Err("trailing bytes after the last nal".to_owned());
    }

    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn one_nal_gains_a_start_code() {
        let avcc = [0, 0, 0, 3, 0x65, 0xAA, 0xBB];
        let out = avcc_to_annex_b(&avcc, 4).expect("converts");
        assert_eq!(&out[..], &[0, 0, 0, 1, 0x65, 0xAA, 0xBB]);
    }

    #[test]
    fn every_nal_in_an_access_unit_gains_one() {
        let avcc = [0, 0, 0, 2, 0x67, 0x11, 0, 0, 0, 2, 0x68, 0x22];
        let out = avcc_to_annex_b(&avcc, 4).expect("converts");
        assert_eq!(&out[..], &[0, 0, 0, 1, 0x67, 0x11, 0, 0, 0, 1, 0x68, 0x22]);
    }

    #[test]
    fn a_length_past_the_end_is_refused_rather_than_truncated() {
        let avcc = [0, 0, 0, 9, 0x65, 0xAA];
        assert!(avcc_to_annex_b(&avcc, 4).is_err());
    }

    #[test]
    fn a_short_trailing_fragment_is_refused() {
        let avcc = [0, 0, 0, 1, 0x65, 0x00, 0x00];
        assert!(avcc_to_annex_b(&avcc, 4).is_err());
    }

    #[test]
    fn an_empty_access_unit_converts_to_nothing() {
        let out = avcc_to_annex_b(&[], 4).expect("converts");
        assert!(out.is_empty());
    }
}
