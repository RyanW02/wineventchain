use crate::{AgentError, Result};
use duration_str::parse;
use std::time::Duration;

pub fn parse_durations<'a, I>(durations: I) -> Result<Vec<Duration>>
where
    I: IntoIterator<Item = &'a String>,
{
    durations
        .into_iter()
        .map(|s| parse(s.as_str()).map_err(AgentError::from))
        .collect()
}
