use crate::model::events::EventWithData;
use crate::{AgentError, Config, Result};
use lazy_static::lazy_static;
use log::error;
use parking_lot::RwLock;
use std::ffi::c_void;
use std::ptr::null_mut;
use tokio::sync::mpsc;
use windows::core::imp::GetLastError;
use windows::core::PCWSTR;
use windows::Win32::Foundation;
use windows::Win32::Foundation::{ERROR_INSUFFICIENT_BUFFER, ERROR_SUCCESS};
use windows::Win32::System::EventLog;
use windows::Win32::System::EventLog::{EvtRender, EVT_HANDLE, EVT_SUBSCRIBE_NOTIFY_ACTION};

mod channel;

pub use channel::Channel;

lazy_static! {
    static ref EVENT_TX: RwLock<Option<mpsc::UnboundedSender<EventWithData>>> = RwLock::new(None);
}

pub struct ChannelCollector {
    pub channel: Channel,
    handle: Option<EVT_HANDLE>,
}

impl ChannelCollector {
    pub fn new(channel: Channel) -> Self {
        Self {
            channel,
            handle: None,
        }
    }

    pub fn init_callback() -> mpsc::UnboundedReceiver<EventWithData> {
        let (tx, rx) = mpsc::unbounded_channel();

        let mut guard = EVENT_TX.write();
        if guard.is_some() {
            panic!("Callback already initialized");
        }

        *guard = Some(tx);

        rx
    }

    /// # Safety
    /// This function is unsafe as it calls the [EventLog::EvtSubscribe] function from the Win32 API.
    pub unsafe fn collect(&mut self, config: &Config) -> Result<()> {
        let channel_path: PCWSTR = self.channel.into();
        let session: EVT_HANDLE = Default::default();
        let signal_event: Foundation::HANDLE = Default::default();
        let bookmark: EVT_HANDLE = Default::default();

        let flags = if config.retrieve_past_events {
            EventLog::EvtSubscribeStartAtOldestRecord.0
        } else {
            EventLog::EvtSubscribeToFutureEvents.0
        };

        let handle = EventLog::EvtSubscribe(
            session,
            signal_event,
            channel_path,
            PCWSTR::null(),
            bookmark,
            None,
            Some(Self::callback),
            flags,
        )?;

        self.handle = Some(handle);

        Ok(())
    }

    /// # Safety
    /// This function is unsafe as it calls the [EventLog::EvtClose] function from the Win32 API.
    pub unsafe fn close(self) -> Result<()> {
        match self.handle {
            Some(handle) => Ok(EventLog::EvtClose(handle)?),
            None => Ok(()),
        }
    }

    // Global callback function that handles events for *all* ChannelCollectors.
    // It is not possible to pass state to an `extern` function.
    unsafe extern "system" fn callback(
        action: EVT_SUBSCRIBE_NOTIFY_ACTION,
        _p_context: *const c_void,
        event_handle: EVT_HANDLE,
    ) -> u32 {
        match action {
            EventLog::EvtSubscribeActionError => {
                if event_handle.0 == Foundation::ERROR_EVT_QUERY_RESULT_STALE.0 as isize {
                    error!("The subscription callback was notified that event records are missing");
                } else {
                    error!(
                        "The subscription callback received the following Win32 error: {}",
                        event_handle.0
                    );
                }
            }
            EventLog::EvtSubscribeActionDeliver => {
                let deserialized = match deserialize_event(event_handle) {
                    Ok(event) => event,
                    Err(e) => {
                        error!("Failed to deserialize event: {}", e);
                        return 0;
                    }
                };

                let channel = EVENT_TX.read().clone();
                match channel {
                    Some(tx) => {
                        if let Err(e) = tx.send(deserialized) {
                            error!("Failed to send event to channel: {}", e);
                        }
                    }
                    None => error!("Event callback has not been initialized"),
                }
            }
            _ => error!("Unknown action {}", action.0),
        }

        0
    }
}

unsafe fn deserialize_event(event_handle: EVT_HANDLE) -> Result<EventWithData> {
    let mut buffer_size = 0u32;
    let mut buffer_used = 0u32;

    let context: EVT_HANDLE = Default::default();

    unsafe {
        let _ = EvtRender(
            context,
            event_handle,
            EventLog::EvtRenderEventXml.0,
            buffer_size,
            None,
            &mut buffer_used as *mut u32,
            null_mut(),
        );

        let status = GetLastError();
        if status != ERROR_SUCCESS.0 && status != ERROR_INSUFFICIENT_BUFFER.0 {
            error!("EvtRender failed with {}", status);
            return AgentError::EvtRenderError(status).into();
        }
    };

    buffer_size = buffer_used;
    let mut buf: Vec<u16> = vec![0; buffer_size as usize];

    if !unsafe {
        let context: EVT_HANDLE = Default::default();

        EvtRender(
            context,
            event_handle,
            EventLog::EvtRenderEventXml.0,
            buffer_size,
            Some(buf.as_mut_ptr() as *mut c_void),
            &mut buffer_used,
            null_mut(),
        )
        .is_ok()
    } {
        let status = GetLastError();
        error!("EvtRender failed with {}", status);
        return AgentError::EvtRenderError(status).into();
    }

    let null_terminated = String::from_utf16_lossy(&buf[..]);
    let xml = null_terminated.trim_matches(char::from(0));

    match EventWithData::from_xml_string(xml) {
        Ok(event) => Ok(event),
        Err(e) => {
            error!("Failed to deserialize event: {}\nXML: {}", e, xml);
            e.into()
        }
    }
}
