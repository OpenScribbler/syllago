## Prerequisites

- Aembit tenant with at least one Access Policy configured
- Expected operational schedule for the workload identified (days of week, time range, timezone)

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{TIME_WINDOW_TIMEZONE}}` | Determined by your use case (e.g., `America/Chicago`) — use the workload's operational timezone |
| `{{TIME_WINDOW_DAYS}}` | Days the workload is expected to run (e.g., Monday–Friday) |
| `{{TIME_WINDOW_START}}` | Start time in the selected timezone (e.g., `08:00`) |
| `{{TIME_WINDOW_END}}` | End time in the selected timezone (e.g., `18:00`) |

## Aembit Configuration

1. Navigate to **Access Conditions** and click **+ New**

2. Complete the dialog:
   - **Name**: descriptive label (e.g., `Business Hours Only`)
   - **Description**: optional
   - **Integration**: select **Aembit Time Condition**

3. Under **Conditions**, configure the time window:
   - **Timezone**: select `{{TIME_WINDOW_TIMEZONE}}`
   - For each day in `{{TIME_WINDOW_DAYS}}`, click the **+** icon next to the day name and set the start and end times

4. Click **Save** - at least one day with a time window is required before the condition can be saved

## Verification

- During the configured time window, confirm the workload accesses the protected resource successfully
- Outside the configured time window, confirm access is denied
- Navigate to **Activity** and confirm that denied events are logged with the Time Condition as the reason

## Troubleshooting

- **Save button disabled:** No time windows have been added. Click the **+** icon next to at least one day and configure a time range before saving
- **Access denied during expected hours:** Verify the **Timezone** setting matches the timezone of the clock being used to test. A mismatch between server timezone and condition timezone is the most common cause
- **Access allowed outside hours:** The Time Condition is not attached to the Access Policy. Navigate to **Access Policies**, open the relevant policy, and confirm the Time Condition appears under Access Conditions
- **No Activity log entries:** The workload is not routing through the Aembit Agent Proxy. Verify the proxy is running and outbound requests are intercepted
