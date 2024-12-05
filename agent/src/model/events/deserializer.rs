use crate::model::events::{
    Correlation, Data, EventData, EventWithData, Execution, Guid, Provider, System, TimeCreated,
};
use crate::{AgentError, Result};
use roxmltree::{Document, Node};
use time::format_description::well_known;
use time::OffsetDateTime;

trait FromXML {
    fn from_xml(n: &Node) -> Result<Self>
    where
        Self: Sized;
}

impl EventWithData {
    pub fn from_xml_string(s: &str) -> Result<Self> {
        let doc = Document::parse(s)?;
        Self::from_xml(&doc.root_element())
    }
}

impl FromXML for EventWithData {
    fn from_xml(n: &Node) -> Result<Self> {
        Ok(Self {
            system: System::from_xml(&get_required_child(n, "System")?)?,
            event_data: get_child(n, "EventData")
                .map(|data_node| EventData::from_xml(&data_node))
                .transpose()?
                .unwrap_or_else(|| EventData(Vec::new())),
        })
    }
}

impl FromXML for System {
    fn from_xml(n: &Node) -> Result<Self> {
        Ok(Self {
            provider: Provider::from_xml(&get_required_child(n, "Provider")?)?,
            event_id: get_required_child(n, "EventID")?
                .text()
                .ok_or_else(|| AgentError::MissingAttribute("EventID".to_string()))?
                .parse()?,
            time_created: TimeCreated::from_xml(&get_required_child(n, "TimeCreated")?)?,
            event_record_id: get_required_child(n, "EventRecordID")?
                .text()
                .ok_or_else(|| AgentError::MissingAttribute("EventRecordID".to_string()))?
                .parse()?,
            correlation: Correlation::from_xml(&get_required_child(n, "Correlation")?)?,
            execution: Execution::from_xml(&get_required_child(n, "Execution")?)?,
            channel: get_required_child(n, "Channel")?
                .text()
                .ok_or_else(|| AgentError::MissingAttribute("Channel".to_string()))?
                .to_string(),
            computer: get_required_child(n, "Computer")?
                .text()
                .ok_or_else(|| AgentError::MissingAttribute("Computer".to_string()))?
                .to_string(),
        })
    }
}

impl FromXML for Provider {
    fn from_xml(n: &Node) -> Result<Self> {
        let name = n.attribute("Name").map(|s| s.to_string());
        let guid = n.attribute("Guid").map(Guid::parse).transpose()?;
        let event_source_name = n.attribute("EventSourceName").map(|s| s.to_string());

        Ok(Self {
            name,
            guid,
            event_source_name,
        })
    }
}

impl FromXML for TimeCreated {
    fn from_xml(n: &Node) -> Result<Self> {
        let system_time = n
            .attribute("SystemTime")
            .ok_or_else(|| AgentError::MissingAttribute("SystemTime".to_string()))?;

        Ok(Self {
            system_time: OffsetDateTime::parse(system_time, &well_known::Rfc3339)?,
        })
    }
}

impl FromXML for Correlation {
    fn from_xml(n: &Node) -> Result<Self> {
        Ok(Self {
            activity_id: n
                .attribute("ActivityID")
                .map(|s| s.parse::<Guid>())
                .transpose()?,
        })
    }
}

impl FromXML for Execution {
    fn from_xml(n: &Node) -> Result<Self> {
        Ok(Self {
            process_id: n
                .attribute("ProcessID")
                .map(|s| s.parse::<usize>())
                .transpose()?,
            thread_id: n
                .attribute("ThreadID")
                .map(|s| s.parse::<usize>())
                .transpose()?,
        })
    }
}

impl FromXML for EventData {
    fn from_xml(n: &Node) -> Result<Self> {
        n.children()
            .filter(|n| n.is_element() && n.tag_name().name() == "Data")
            .map(|n| Data::from_xml(&n))
            .collect::<Result<Vec<Data>>>()
            .map(EventData)
    }
}

impl FromXML for Data {
    fn from_xml(n: &Node) -> Result<Self> {
        let name = n.attribute("Name").map(|s| s.to_string());
        let value = n.text().map(|s| s.to_string());

        Ok(Self { name, value })
    }
}

fn get_child<'a>(n: &'a Node<'a, 'a>, name: &'a str) -> Option<Node<'a, 'a>> {
    n.children()
        .find(|n| n.is_element() && n.tag_name().name() == name)
}

fn get_required_child<'a>(n: &'a Node<'a, 'a>, name: &'a str) -> Result<Node<'a, 'a>> {
    get_child(n, name).ok_or_else(|| AgentError::MissingAttribute(name.to_string()))
}

#[cfg(test)]
mod tests {
    use super::*;
    use time::Month;

    fn test_node<T, F>(s: &str, f: F)
    where
        T: FromXML,
        F: FnOnce(T),
    {
        let doc = Document::parse(s).unwrap();
        let node = doc.root_element();
        let deserialized = T::from_xml(&node).unwrap();
        f(deserialized)
    }

    #[test]
    fn test_deserialize_data() {
        let data = r#"<Data Name="SubjectDomainName">NT AUTHORITY</Data>"#;

        test_node(data, |data: Data| {
            assert_eq!(
                data,
                Data {
                    name: Some("SubjectDomainName".to_string()),
                    value: Some("NT AUTHORITY".to_string()),
                }
            );
        });
    }

    #[test]
    fn test_deserialize_event_data() {
        let data = r#"
<EventData>
    <Data Name="SubjectDomainName">NT AUTHORITY</Data>
    <Data Name="SubjectUserName">SYSTEM</Data>
    <Data Name="SubjectLogonId">0x3e7</Data>
</EventData>
"#;

        test_node(data, |data: EventData| {
            assert_eq!(
                data,
                EventData(vec![
                    Data {
                        name: Some("SubjectDomainName".to_string()),
                        value: Some("NT AUTHORITY".to_string()),
                    },
                    Data {
                        name: Some("SubjectUserName".to_string()),
                        value: Some("SYSTEM".to_string()),
                    },
                    Data {
                        name: Some("SubjectLogonId".to_string()),
                        value: Some("0x3e7".to_string()),
                    },
                ])
            );
        });
    }

    #[test]
    fn test_deserialize_execution() {
        let data = r#"<Execution ProcessID="4" ThreadID="8" />"#;

        test_node(data, |data: Execution| {
            assert_eq!(
                data,
                Execution {
                    process_id: Some(4),
                    thread_id: Some(8),
                }
            );
        });
    }

    #[test]
    fn test_deserialize_execution_empty() {
        let data = r#"<Execution/>"#;

        test_node(data, |data: Execution| {
            assert_eq!(
                data,
                Execution {
                    process_id: None,
                    thread_id: None,
                }
            );
        });
    }

    #[test]
    fn test_deserialize_correlation() {
        let data = r#"<Correlation ActivityID="{6286c69f-b8c2-4890-96c2-33b5421c748e}" />"#;

        test_node(data, |data: Correlation| {
            assert_eq!(
                data.activity_id,
                Some(Guid::parse("6286c69f-b8c2-4890-96c2-33b5421c748e").unwrap())
            );
        });
    }

    #[test]
    fn test_deserialize_correlation_empty() {
        let data = r#"<Correlation/>"#;

        test_node(data, |data: Correlation| {
            assert_eq!(data.activity_id, None);
        });
    }

    #[test]
    fn test_deserialize_time_created() {
        let data = r#"<TimeCreated SystemTime="2024-02-16T13:24:41.8547517Z" />"#;

        test_node(data, |data: TimeCreated| {
            assert_eq!(data.system_time.year(), 2024);
            assert_eq!(data.system_time.month(), Month::February);
            assert_eq!(data.system_time.day(), 16);
            assert_eq!(data.system_time.hour(), 13);
            assert_eq!(data.system_time.minute(), 24);
            assert_eq!(data.system_time.second(), 41);
            assert_eq!(data.system_time.nanosecond(), 854751700);
        });
    }

    #[test]
    fn test_provider_full() {
        let data = r#"<Provider Name="Microsoft-Windows-Security-Auditing" Guid="{08f0936b-c179-465a-874e-e2a5a7e2f638}" EventSourceName="abc" />"#;

        test_node(data, |data: Provider| {
            assert_eq!(
                data,
                Provider {
                    name: Some("Microsoft-Windows-Security-Auditing".to_string()),
                    guid: Some(Guid::parse("08f0936b-c179-465a-874e-e2a5a7e2f638").unwrap()),
                    event_source_name: Some("abc".to_string()),
                }
            );
        });
    }

    #[test]
    fn test_provider_empty() {
        let data = r#"<Provider />"#;

        test_node(data, |data: Provider| {
            assert_eq!(
                data,
                Provider {
                    name: None,
                    guid: None,
                    event_source_name: None,
                }
            );
        });
    }

    #[test]
    fn test_provider_name_and_guid() {
        let data = r#"<Provider Name="Microsoft-Windows-Security-Auditing" Guid="{08f0936b-c179-465a-874e-e2a5a7e2f638}" />"#;

        test_node(data, |data: Provider| {
            assert_eq!(
                data,
                Provider {
                    name: Some("Microsoft-Windows-Security-Auditing".to_string()),
                    guid: Some(Guid::parse("08f0936b-c179-465a-874e-e2a5a7e2f638").unwrap()),
                    event_source_name: None,
                }
            );
        });
    }

    #[test]
    fn test_system() {
        let data = r#"
<System>
  <Provider Name="Microsoft-Windows-Security-Auditing" Guid="{08f0936b-c179-465a-874e-e2a5a7e2f638}" /> 
  <EventID>5379</EventID> 
  <Version>0</Version> 
  <Level>0</Level> 
  <Task>13824</Task> 
  <Opcode>0</Opcode> 
  <Keywords>0x8020000000000000</Keywords> 
  <TimeCreated SystemTime="2024-02-20T00:59:00.2324235Z" /> 
  <EventRecordID>677930</EventRecordID> 
  <Correlation ActivityID="{5b3bdff3-35b9-44dd-844d-4e193236c42e}" /> 
  <Execution ProcessID="3436" ThreadID="32400" /> 
  <Channel>Security</Channel> 
  <Computer>laptop</Computer> 
  <Security /> 
</System>
"#;

        test_node(data, |data: System| {
            assert_eq!(
                data,
                System {
                    provider: Provider {
                        name: Some("Microsoft-Windows-Security-Auditing".to_string()),
                        guid: Some(Guid::parse("08f0936b-c179-465a-874e-e2a5a7e2f638").unwrap()),
                        event_source_name: None,
                    },
                    event_id: 5379,
                    time_created: TimeCreated {
                        system_time: OffsetDateTime::parse(
                            "2024-02-20T00:59:00.2324235Z",
                            &well_known::Rfc3339
                        )
                        .unwrap(),
                    },
                    event_record_id: 677930,
                    correlation: Correlation {
                        activity_id: Some(
                            Guid::parse("5b3bdff3-35b9-44dd-844d-4e193236c42e").unwrap()
                        ),
                    },
                    execution: Execution {
                        process_id: Some(3436),
                        thread_id: Some(32400),
                    },
                    channel: "Security".to_string(),
                    computer: "laptop".to_string(),
                }
            );
        });
    }

    #[test]
    fn test_event_with_data() {
        let data = r#"
<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">
    <System>
        <Provider Name="Microsoft-Windows-Kernel-Power" Guid="{7871afc8-b522-42ab-a77c-40709a08d7e1}" /> 
        <EventID>130</EventID> 
        <Version>0</Version> 
        <Level>4</Level> 
        <Task>33</Task> 
        <Opcode>0</Opcode> 
        <Keywords>0x8000000000000404</Keywords> 
        <TimeCreated SystemTime="2024-02-20T00:55:57.5003944Z" /> 
        <EventRecordID>71483</EventRecordID> 
        <Correlation /> 
        <Execution ProcessID="4" ThreadID="27912" /> 
        <Channel>System</Channel> 
        <Computer>laptop</Computer> 
        <Security /> 
    </System>
    <EventData>
        <Data Name="SuspendStart">123</Data> 
        <Data Name="SuspendEnd">456</Data> 
    </EventData>
</Event>
"#;

        test_node(data, |data: EventWithData| {
            assert_eq!(
                data,
                EventWithData {
                    system: System {
                        provider: Provider {
                            name: Some("Microsoft-Windows-Kernel-Power".to_string()),
                            guid: Some(
                                Guid::parse("7871afc8-b522-42ab-a77c-40709a08d7e1").unwrap()
                            ),
                            event_source_name: None,
                        },
                        event_id: 130,
                        time_created: TimeCreated {
                            system_time: OffsetDateTime::parse(
                                "2024-02-20T00:55:57.5003944Z",
                                &well_known::Rfc3339
                            )
                            .unwrap(),
                        },
                        event_record_id: 71483,
                        correlation: Correlation { activity_id: None },
                        execution: Execution {
                            process_id: Some(4),
                            thread_id: Some(27912),
                        },
                        channel: "System".to_string(),
                        computer: "laptop".to_string(),
                    },
                    event_data: EventData(vec![
                        Data {
                            name: Some("SuspendStart".to_string()),
                            value: Some("123".to_string()),
                        },
                        Data {
                            name: Some("SuspendEnd".to_string()),
                            value: Some("456".to_string()),
                        },
                    ],)
                }
            );
        });
    }
}
