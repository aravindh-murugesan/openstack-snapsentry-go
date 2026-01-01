package openstack

import "github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"

type VMGroupedVolumeList struct {
	Attached      map[string][]volumes.Volume
	MultiAttached []volumes.Volume
	Unattached    []volumes.Volume
}
