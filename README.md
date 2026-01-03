# OpenStack SnapSentry Go 🛡️

A robust, policy and metadata-driven snapshot management tool for OpenStack Cinder, written in Go.

## Features

* **Metadata Driven:** No central configuration file or database required. Define backup schedules directly on the volume metadata.
* **Flexible Policies:**
    * **Express:** Multiple snapshots per day (Supported Intervals: 6, 8, 12 hours).
    * **Daily, Weekly, Monthly:** Standard retention schedules.
* **Atomic VM Snapshots:** Automatically groups volumes attached to the same VM and snapshots them simultaneously (simulating consistency across disks).
* **Hybrid Concurrency:**
    * *Attached Volumes:* Processed concurrently for speed.
    * *Unattached/Shared Volumes:* Processed sequentially to prevent API throttling and ensure safety.
* **Idempotency**: Ensure no duplicate snapshots are created for a specific snapshot window. 
* **Self-Healing:** Built-in retry logic for transient OpenStack errors (HTTP 500s/Network issues) and automatic cleanup of orphaned "zombie" snapshots.

## Installation

### Build from Source

```bash
git clone [https://github.com/aravindh-murugesan/openstack-snapsentry-go.git](https://github.com/aravindh-murugesan/openstack-snapsentry-go.git)
cd openstack-snapsentry-go
go build -o snapsentry-go cmd/main.go
```

## Configuration

SnapSentry Go relies on the Gophercloud SDK and works with the standard OpenStack `clouds.yaml` configuration file.

- Create `~/.config/openstack/clouds.yaml` (or `/etc/openstack/clouds.yaml`) with your cloud profiles.
Note: For more information on authentication, please refer to the [OpenStack Client Configuration Documentation](https://docs.openstack.org/python-openstackclient/latest/configuration/index.html#configuration-files).

## Security Best Practice: Restricted Application Credentials

It is highly recommended to use an Application Credential with restricted access. For operations like force-deleting snapshots stuck in an creating state, the credential requires a role with appropriate permissions (e.g., `admin` or a custom role), but endpoint access should be strictly limited to the Block Storage service to maintain a Least Privilege philosophy.

**1. Create Access Rules**

Save the following JSON to a file named access.json. This restricts snapsentry to only interact with the Volume service (Cinder).

```json
[
  {
    "service": "volumev3",
    "method": "GET",
    "path": "/v3/**"
  },
  {
    "service": "volumev3",
    "method": "POST",
    "path": "/v3/{project_id}/snapshots/**"
  },
  {
    "service": "volumev3",
    "method": "DELETE",
    "path": "/v3/{project_id}/snapshots/**"
  },
  {
    "service": "volumev3",
    "method": "PUT",
    "path": "/v3/{project_id}/snapshots/**"
  }
]
```

**2. Generate the Credential**

Run the following command using an existing admin profile to create the restricted credential:

```bash
openstack --os-cloud <existing-admin-profile> application credential create snapsentry-bot \
  --description "Restricted credential for SnapSentry backups" \
  --access-rules access.json \
  --role admin \
  --restricted
```

**3. Update clouds.yaml**

Add the new profile to your configuration file:

```yaml
clouds:
  snapsentry-bot:
    auth:
      auth_url: <KEYSTONE_ENDPOINT>
      application_credential_id: <YOUR_APP_ID>
      application_credential_secret: <YOUR_APP_SECRET>
    region_name: <REGION_NAME>
    interface: public
    identity_api_version: 3
    auth_type: v3applicationcredential
    timeout: 10
    verify: true
```

## Usage

You can run SnapSentry in Daemon Mode (standalone) or CLI Mode (for integration with external schedulers like Argo Workflows or Kubernetes CronJobs).

**1. Tag Your Volumes**

Enable snapshot policies by setting specific metadata on your OpenStack volumes. We provide helper commands to make this easier.

```bash
# Configure a Daily Policy (Keep 1 snapshot, taken at 08:00 IST)
snapsentry-go --cloud snapsentry-bot subscribe daily \
  --start-time 08:00 --timezone "Asia/Kolkata" \
  --retention 1 --volume-id "<VOLUME-ID>"

# Configure a Weekly Policy (Run on Sundays at 23:00 CET, keep 2)
snapsentry-go --cloud snapsentry-bot subscribe weekly \
  --timezone "Europe/Berlin" --start-time 23:00 \
  --retention 2 --week-day sunday --volume-id "<VOLUME-ID>"

# Configure a Monthly Policy (Run on the 1st at 23:00 CET, keep 2)
snapsentry-go --cloud snapsentry-bot subscribe monthly \
  --timezone "Europe/Berlin" --start-time 23:00 \
  --retention 2 --month-day 1 --volume-id "<VOLUME-ID>"

# Configure an Express Policy (Run every 6 hours)
snapsentry-go --cloud snapsentry-bot subscribe express \
  --timezone "Europe/Berlin" --retention 2 \
  --interval-hours 6 --volume-id "<VOLUME-ID>"
```

**2. Run SnapSentry**

**CLI Mode (One off execution)**
```bash
# Trigger snapshot creation process
snapsentry-go create-snapshots --cloud snapsentry --log-level info

# Trigger snapshot cleanup process
snapsentry-go expire-snapshots --cloud snapsentry --log-level info
```

**Daemon Mode (Continuous)**
Runs continuously and executes tasks based on the provided Cron schedules.

```bash
./snapsentry daemon --cloud snapsentry-bot \
  --bind-address 127.0.0.1:4005 \ 
  --create-schedule "*/10 * * * *" \
  --expire-schedule "*/30 * * * *"
```
