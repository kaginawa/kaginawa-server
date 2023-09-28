package kaginawa

import (
	"fmt"
	"time"

	"github.com/kaginawa/kaginawa-server/internal/database"
)

// Report defines all Report attributes
type Report struct {
	// Kaginawa shared fields
	ID             string      `json:"id" bson:"id"`                                       // MAC address
	Trigger        int         `json:"trigger,omitempty" bson:"trigger"`                   // Report trigger (-1/0/n)
	Runtime        string      `json:"runtime,omitempty" bson:"runtime"`                   // OS and arch
	Success        bool        `json:"success" bson:"success"`                             // Equals len(Errors) == 0
	Sequence       int         `json:"seq,omitempty" bson:"seq"`                           // Report sequence number
	DeviceTime     int64       `json:"device_time,omitempty" bson:"device_time"`           // Device time (UTC)
	BootTime       int64       `json:"boot_time,omitempty" bson:"boot_time"`               // Device boot time (UTC)
	GenMillis      int64       `json:"gen_ms,omitempty" bson:"gen_ms"`                     // Generation time millis
	AgentVersion   string      `json:"agent_version,omitempty" bson:"agent_version"`       // Agent version
	CustomID       string      `json:"custom_id,omitempty" bson:"custom_id"`               // User specified ID
	SSHServerHost  string      `json:"ssh_server_host,omitempty" bson:"ssh_server_host"`   // Connected SSH server host
	SSHRemotePort  int         `json:"ssh_remote_port,omitempty" bson:"ssh_remote_port"`   // Connected SSH remote port
	SSHConnectTime int64       `json:"ssh_connect_time,omitempty" bson:"ssh_connect_time"` // Connected time of the SSH
	Adapter        string      `json:"adapter,omitempty" bson:"adapter"`                   // Name of network adapter
	LocalIPv4      string      `json:"ip4_local,omitempty" bson:"ip4_local"`               // Local IPv6 address
	LocalIPv6      string      `json:"ip6_local,omitempty" bson:"ip6_local"`               // Local IPv6 address
	Hostname       string      `json:"hostname,omitempty" bson:"hostname"`                 // OS Hostname
	RTTMills       int64       `json:"rtt_ms,omitempty" bson:"rtt_ms"`                     // Round trip time millis
	UploadKBPS     int64       `json:"upload_bps,omitempty" bson:"upload_bps"`             // Upload throughput bps
	DownloadKBPS   int64       `json:"download_bps,omitempty" bson:"download_bps"`         // Download throughput bps
	DiskTotalBytes int64       `json:"disk_total_bytes,omitempty" bson:"disk_total_bytes"` // Total disk space (Bytes)
	DiskUsedBytes  int64       `json:"disk_used_bytes,omitempty" bson:"disk_used_bytes"`   // Used disk space (Bytes)
	DiskLabel      string      `json:"disk_label,omitempty" bson:"disk_label"`             // Disk label
	DiskFilesystem string      `json:"disk_filesystem,omitempty" bson:"disk_filesystem"`   // Disk filesystem name
	DiskMountPoint string      `json:"disk_mount_point,omitempty" bson:"disk_mount_point"` // Mount point (default is /)
	DiskDevice     string      `json:"disk_device,omitempty" bson:"disk_device"`           // Disk device name
	USBDevices     []USBDevice `json:"usb_devices,omitempty" bson:"usb_devices"`           // List of usb devices
	BDLocalDevices []string    `json:"bd_local_devices,omitempty" bson:"bd_local_devices"` // List of BT local devices
	KernelVersion  string      `json:"kernel_version,omitempty" bson:"kernel_version"`     // Kernel version
	Errors         []string    `json:"errors,omitempty" bson:"errors"`                     // List of errors
	Payload        string      `json:"payload,omitempty" bson:"payload"`                   // Custom content
	PayloadCmd     string      `json:"payload_cmd,omitempty" bson:"payload_cmd"`           // Executed payload command

	// Server-side injected fields
	GlobalIP   string    `json:"ip_global,omitempty" bson:"ip_global"`     // Global IP address
	GlobalHost string    `json:"host_global,omitempty" bson:"host_global"` // Reverse lookup result for global IP address
	ServerTime int64     `json:"server_time" bson:"server_time"`           // Server-side consumed UTC time
	APIKey     string    `json:"api_key,omitempty" bson:"api_key"`         // Used api key
	TTL        time.Time `json:"-" bson:"-" dynamodbav:",unixtime"`        // DynamoDB TTL
}

// DownloadMBPS formats download throughput as Mbps.
func (r Report) DownloadMBPS() string {
	return fmt.Sprintf("%.1f", float64(r.DownloadKBPS)/1024)
}

// UploadMBPS formats upload throughput as Mbps.
func (r Report) UploadMBPS() string {
	return fmt.Sprintf("%.1f", float64(r.UploadKBPS)/1024)
}

// DiskUsageAsPercentage calculates disk usage as percentage.
func (r Report) DiskUsageAsPercentage() string {
	if r.DiskTotalBytes == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.1f%%", float64(r.DiskUsedBytes)/float64(r.DiskTotalBytes)*100)
}

// IsBootTimeReport checks report triggered by boot time or not.
func (r Report) IsBootTimeReport() bool {
	return r.Trigger == 0
}

// IsSSHConnectedReport checks report triggered by ssh connected or not.
func (r Report) IsSSHConnectedReport() bool {
	return r.Trigger == -1
}

// IsIntervalReport checks report triggered by interval timer or not.
func (r Report) IsIntervalReport() bool {
	return r.Trigger > 0
}

// USBDevice defines usb device information
type USBDevice struct {
	Name      string `json:"name,omitempty" bson:"name"`
	VendorID  string `json:"vendor_id,omitempty" bson:"vendor_id"`
	ProductID string `json:"product_id,omitempty" bson:"product_id"`
	Location  string `json:"location,omitempty" bson:"location"`
}

// MatchReports generates list of reports filtered by specified matcher function.
func MatchReports(db database.DB, minutes int, projection database.Projection, matcher func(r Report) bool) ([]Report, error) {
	reports, err := db.ListReports(0, 0, minutes, projection)
	if err != nil {
		return nil, err
	}
	var matches []Report
	for _, report := range reports {
		if matcher(report) {
			matches = append(matches, report)
		}
	}
	return matches, nil
}

// SubReports generates subsets of source reports.
func SubReports(reports []Report, skip, limit int) (sub []Report) {
	for i, report := range reports {
		if i < skip {
			continue
		}
		if limit > 0 && len(sub) >= limit {
			break
		}
		sub = append(sub, report)
	}
	return
}
