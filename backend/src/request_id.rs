use std::collections::BTreeMap;
use std::fmt;

use uuid::Uuid;

pub const REQUEST_ID_HEADER: &str = "X-Request-Id";
pub const MAX_REQUEST_ID_LEN: usize = 127;

#[derive(Clone, Debug, Eq, Hash, PartialEq)]
pub struct RequestId(String);

impl RequestId {
    pub fn from_header_value(value: Option<&str>) -> Self {
        value
            .map(str::trim)
            .filter(|value| is_valid_request_id(value))
            .map(|value| Self(value.to_owned()))
            .unwrap_or_else(Self::generate)
    }

    pub fn from_headers(headers: &BTreeMap<String, String>) -> Self {
        let header_value = headers
            .iter()
            .find(|(name, _)| name.eq_ignore_ascii_case(REQUEST_ID_HEADER))
            .map(|(_, value)| value.as_str());

        Self::from_header_value(header_value)
    }

    pub fn generate() -> Self {
        Self(Uuid::new_v4().to_string())
    }

    pub fn as_str(&self) -> &str {
        &self.0
    }

    pub fn insert_response_header(&self, headers: &mut BTreeMap<String, String>) {
        headers.insert(REQUEST_ID_HEADER.to_owned(), self.0.clone());
    }

    pub fn log_request(&self, method: &str, path: &str) {
        tracing::info!(
            request_id = %self,
            method = method,
            path = path,
            "backend request received"
        );
    }

    pub fn log_response(&self, method: &str, path: &str, status_code: u16) {
        tracing::info!(
            request_id = %self,
            method = method,
            path = path,
            status_code = status_code,
            "backend response sent"
        );
    }
}

impl fmt::Display for RequestId {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(&self.0)
    }
}

fn is_valid_request_id(value: &str) -> bool {
    !value.is_empty() && value.len() <= MAX_REQUEST_ID_LEN && !value.chars().any(char::is_control)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn assert_generated_uuid(value: &RequestId) {
        Uuid::parse_str(value.as_str()).expect("generated request id should be a UUID");
    }

    #[test]
    fn provided_request_id_is_preserved_and_returned() {
        let request_id = RequestId::from_header_value(Some("client-req-123"));
        let mut response_headers = BTreeMap::new();

        request_id.insert_response_header(&mut response_headers);

        assert_eq!(request_id.as_str(), "client-req-123");
        assert_eq!(
            response_headers.get(REQUEST_ID_HEADER).map(String::as_str),
            Some("client-req-123")
        );
    }

    #[test]
    fn missing_request_id_generates_uuid_response_header() {
        let request_id = RequestId::from_header_value(None);
        let mut response_headers = BTreeMap::new();

        request_id.insert_response_header(&mut response_headers);

        assert_generated_uuid(&request_id);
        assert_eq!(
            response_headers.get(REQUEST_ID_HEADER).map(String::as_str),
            Some(request_id.as_str())
        );
    }

    #[test]
    fn invalid_request_ids_generate_uuid_values() {
        let invalid_values = [
            String::new(),
            "   ".to_owned(),
            "contains\nnewline".to_owned(),
            "x".repeat(MAX_REQUEST_ID_LEN + 1),
        ];

        for invalid_value in invalid_values {
            let request_id = RequestId::from_header_value(Some(&invalid_value));

            assert_ne!(request_id.as_str(), invalid_value.trim());
            assert_generated_uuid(&request_id);
        }
    }

    #[test]
    fn request_id_header_lookup_is_case_insensitive() {
        let mut headers = BTreeMap::new();
        headers.insert("x-request-id".to_owned(), "case-insensitive-id".to_owned());

        let request_id = RequestId::from_headers(&headers);

        assert_eq!(request_id.as_str(), "case-insensitive-id");
    }
}
