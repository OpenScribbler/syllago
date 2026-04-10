## Overview

Aembit retains the last 24 hours of activity in the Management Console reporting tab. Configuring a Splunk Log Stream exports Audit Logs, Access Authorization Events, and Workload Events to your Splunk instance for long-term retention and analysis.

## Prerequisites

- Splunk instance with HTTP Event Collector (HEC) enabled
- HEC configured with source type **generic_single_line** (under Miscellaneous category)
- HEC Token Value and Source Name from Splunk
- Aembit tenant with Administrator access

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{SPLUNK_HOST}}` | Splunk instance hostname or IP address |
| `{{SPLUNK_PORT}}` | Splunk HEC port (typically `8088`) |
| `{{SPLUNK_HEC_TOKEN}}` | Splunk → **Settings → Data Inputs → HTTP Event Collector** → select the HEC input → **Token Value** |
| `{{SPLUNK_SOURCE_NAME}}` | Splunk → **Settings → Data Inputs → HTTP Event Collector** → select the HEC input → **Source Name** |

## Aembit Configuration

1. In the Aembit Management Console, navigate to **Administration → Log Streams** and click **+ New**
   - **Name**: descriptive label (e.g., `Splunk - Access Events`)
   - **Description**: optional
   - **Event Type**: select the event type to export (**Audit Logs**, **Access Authorization Events**, or **Workload Events**)
   - **Destination Type**: select **Splunk SIEM using Http Event Collector (HEC)**
   - **Splunk Host/Port**: `{{SPLUNK_HOST}}` and `{{SPLUNK_PORT}}`
   - **TLS**: enable if your Splunk HEC endpoint uses HTTPS
   - **Authentication Token**: `{{SPLUNK_HEC_TOKEN}}`
   - **Source Name**: `{{SPLUNK_SOURCE_NAME}}`
   - Click **Save**

2. Repeat step 1 for each additional event type (each event type requires a separate Log Stream entry)

## Verification

- Each Log Stream entry shows as **Active** in the **Administration → Log Streams** list
- Perform a test action in the Aembit Management Console (e.g., view an Access Policy) to generate an Audit Log event
- In Splunk, navigate to **Search and Reporting** and run: `source={{SPLUNK_SOURCE_NAME}}`
- Confirm events appear within a few minutes

## Troubleshooting

- **Log Stream shows Inactive or error status:** The HEC token is incorrect or the Splunk endpoint is unreachable. Verify the token matches the value in Splunk and confirm the host and port are accessible from the internet
- **No events appear in Splunk:** The source name does not match. Verify the **Source Name** in the Aembit Log Stream matches the HEC input source name in Splunk exactly
- **TLS connection errors:** The Splunk HEC endpoint requires TLS but it is not enabled in the Log Stream, or the certificate is not trusted. Enable **TLS** and check **TLS Verification** settings
- **Events missing for a specific type:** Each event type (Audit, Authorization, Workload) requires its own Log Stream entry. Confirm a Log Stream exists for the event type you expect to see
