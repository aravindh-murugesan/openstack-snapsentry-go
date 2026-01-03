#!/usr/bin/env bash
#
# SnapSentry OpenStack Metadata Setup
# One-time script to configure metadata definitions for snapshot lifecycle management
#

readonly NS="OS::Cinder::Snapsentry"

# Check if uvx mode is requested
USE_UVX=false
if [[ "${1:-}" == "uvx" ]]; then
    USE_UVX=true
    shift
fi

# Alias openstack command based on mode
if [[ "$USE_UVX" == true ]]; then
    openstack() {
        uvx --from python-openstackclient openstack --insecure "$@"
    }
else
    openstack() {
        command openstack --insecure "$@"
    }
fi

# Create a metadata property
create_property() {
    local name="$1"
    local title="$2"
    local type="$3"
    local schema="$4"

    openstack image metadef property create "$NS" \
        --name "$name" \
        --title "$title" \
        --type "$type" \
        --schema "$schema"
}

echo "Creating namespace..."
openstack image metadef namespace create \
    "$NS" \
    --display-name "SnapSentry - Snapshot Lifecycle Manager" \
    --description "Configure automated snapshot lifecycles for this volume." \
    --public \
    --protected

echo "Associating resource type..."
openstack image metadef resource type association create "$NS" OS::Cinder::Volume

echo "Creating metadata properties..."

# General
create_property x-snapsentry-managed "Enable SnapSentry" boolean \
    '{"description":"If set to true, SnapSentry will manage snapshots for this volume.","default":true}'

# Daily schedule
create_property x-snapsentry-daily-enabled "Enable Daily Schedule" boolean '{"default":false}'
create_property x-snapsentry-daily-retention-days "Daily Retention (Days)" integer '{"minimum":1,"default":1}'
create_property x-snapsentry-daily-retention-type "Daily Retention Logic" string '{"enum":["time"],"default":"time"}'
create_property x-snapsentry-daily-timezone "Daily Timezone" string '{"default":"UTC"}'
create_property x-snapsentry-daily-start-time "Daily Start Time" string '{"pattern":"^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$","default":"00:00"}'

# Weekly schedule
create_property x-snapsentry-weekly-enabled "Enable Weekly Schedule" boolean '{"default":false}'
create_property x-snapsentry-weekly-retention-days "Weekly Retention (Days)" integer '{"minimum":7,"default":7}'
create_property x-snapsentry-weekly-retention-type "Weekly Retention Logic" string '{"enum":["time"],"default":"time"}'
create_property x-snapsentry-weekly-timezone "Weekly Timezone" string '{"default":"UTC"}'
create_property x-snapsentry-weekly-start-time "Weekly Start Time" string '{"pattern":"^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$","default":"00:00"}'
create_property x-snapsentry-weekly-start-day-of-week "Weekly Day of Week" string '{"enum":["monday","tuesday","wednesday","thursday","friday","saturday","sunday"],"default":"sunday"}'

# Monthly schedule
create_property x-snapsentry-monthly-enabled "Enable Monthly Schedule" boolean '{"default":false}'
create_property x-snapsentry-monthly-retention-days "Monthly Retention (Days)" integer '{"minimum":31,"default":31}'
create_property x-snapsentry-monthly-retention-type "Monthly Retention Logic" string '{"enum":["time"],"default":"time"}'
create_property x-snapsentry-monthly-timezone "Monthly Timezone" string '{"default":"UTC"}'
create_property x-snapsentry-monthly-start-time "Monthly Start Time" string '{"pattern":"^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$","default":"00:00"}'
create_property x-snapsentry-monthly-start-day-of-month "Monthly Day of Month" integer '{"minimum":1,"maximum":31,"default":1}'

# Express schedule
create_property x-snapsentry-express-enabled "Enable Express Schedule" boolean '{"default":false}'
create_property x-snapsentry-express-interval-hours "Express Interval (Hours)" string '{"enum":["6","8","12"],"default":"6"}'
create_property x-snapsentry-express-retention-days "Express Retention (Days)" integer '{"minimum":1,"default":1}'
create_property x-snapsentry-express-retention-type "Express Retention Logic" string '{"enum":["time"],"default":"time"}'
create_property x-snapsentry-express-timezone "Express Timezone" string '{"default":"UTC"}'

echo "Setup completed successfully!"
