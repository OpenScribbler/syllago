## Prerequisites

- Aembit tenant with at least one Access Policy configured
- Client workload's expected source region identified (country and optionally state/province)

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{GEO_ALLOWED_COUNTRY}}` | Determined by your use case (e.g., `United States`) |
| `{{GEO_ALLOWED_SUBDIVISION}}` | State, province, or region within the country (e.g., `California`) — optional |

## Aembit Configuration

1. Navigate to **Access Conditions** and click **+ New**

2. Complete the dialog:
   - **Name**: descriptive label (e.g., `US West Only`)
   - **Description**: optional
   - **Integration**: select **Aembit GeoIP Condition**

> **Note:** For cloud-hosted workloads (Lambda, EC2, containers), restrict to the country level only. Geolocation services report lower confidence for cloud IP addresses at the subdivision level, which may cause unexpected access denials.

3. Under **Conditions → Location**, configure the allowed region:
   - **Country**: select `{{GEO_ALLOWED_COUNTRY}}`
   - **Subdivision**: optionally select `{{GEO_ALLOWED_SUBDIVISION}}` (click **+** to add)

4. Click **Save**

## Verification

- From the allowed region, confirm the workload accesses the protected resource successfully
- From a different country or region (use a VPN or a separate cloud region), confirm access is denied
- Navigate to **Activity** and confirm that denied events are logged with the GeoIP condition as the reason

## Troubleshooting

- **Access denied from the expected region:** The subdivision is too specific for a cloud-hosted workload. Edit the Access Condition and remove the **Subdivision** filter, relying on country-level matching only
- **Access allowed from an unexpected region:** The Access Condition is not attached to the Access Policy. Navigate to **Access Policies**, open the relevant policy, and confirm the GeoIP condition appears under Access Conditions
- **No Activity log entries:** The workload is not routing through the Aembit Agent Proxy. Verify the proxy is running and the application's outbound requests are intercepted
