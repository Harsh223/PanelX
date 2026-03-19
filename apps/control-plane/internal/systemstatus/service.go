package systemstatus

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Service collects VPS health and capacity metrics for the dashboard.
type Service struct {
	sitesRoot     string
	phpSocketPath string
}

// NewService builds a system status collector.
func NewService(sitesRoot, phpSocketPath string) *Service {
	return &Service{
		sitesRoot:     sitesRoot,
		phpSocketPath: phpSocketPath,
	}
}

// Snapshot is the aggregate payload consumed by the web panel dashboard.
type Snapshot struct {
	Status      string                   `json:"status"`
	Timestamp   string                   `json:"timestamp"`
	Hostname    string                   `json:"hostname"`
	OS          string                   `json:"os"`
	Arch        string                   `json:"arch"`
	Kernel      string                   `json:"kernel"`
	Uptime      UptimeStatus             `json:"uptime"`
	Load        LoadStatus               `json:"load"`
	CPU         CPUStatus                `json:"cpu"`
	Memory      MemoryStatus             `json:"memory"`
	Disk        []DiskStatus             `json:"disk"`
	Services    []ManagedServiceStatus   `json:"services"`
	Network     []NetworkInterfaceStatus `json:"network"`
	WordPress   WordPressStatus          `json:"wordpress"`
	Warnings    []string                 `json:"warnings,omitempty"`
	CollectedMs int64                    `json:"collectedMs"`
}

type UptimeStatus struct {
	Seconds float64 `json:"seconds"`
	Human   string  `json:"human"`
}

type LoadStatus struct {
	One     float64 `json:"one"`
	Five    float64 `json:"five"`
	Fifteen float64 `json:"fifteen"`
}

type CPUStatus struct {
	Cores         int     `json:"cores"`
	UsagePercent  float64 `json:"usagePercent"`
	SampleWindow  string  `json:"sampleWindow"`
	UsageMeasured bool    `json:"usageMeasured"`
}

type MemoryStatus struct {
	TotalBytes       uint64  `json:"totalBytes"`
	AvailableBytes   uint64  `json:"availableBytes"`
	UsedBytes        uint64  `json:"usedBytes"`
	UsedPercent      float64 `json:"usedPercent"`
	SwapTotalBytes   uint64  `json:"swapTotalBytes"`
	SwapFreeBytes    uint64  `json:"swapFreeBytes"`
	SwapUsedBytes    uint64  `json:"swapUsedBytes"`
	SwapUsedPercent  float64 `json:"swapUsedPercent"`
	MeasurementUnits string  `json:"measurementUnits"`
}

type DiskStatus struct {
	Path        string  `json:"path"`
	Filesystem  string  `json:"filesystem"`
	TotalBytes  uint64  `json:"totalBytes"`
	UsedBytes   uint64  `json:"usedBytes"`
	FreeBytes   uint64  `json:"freeBytes"`
	UsedPercent float64 `json:"usedPercent"`
}

type ManagedServiceStatus struct {
	Name        string `json:"name"`
	Active      bool   `json:"active"`
	State       string `json:"state"`
	Description string `json:"description,omitempty"`
}

type NetworkInterfaceStatus struct {
	Name           string `json:"name"`
	RxBytes        uint64 `json:"rxBytes"`
	TxBytes        uint64 `json:"txBytes"`
	ReceiveErrors  uint64 `json:"receiveErrors"`
	TransmitErrors uint64 `json:"transmitErrors"`
}

type WordPressStatus struct {
	SitesRoot      string `json:"sitesRoot"`
	DetectedSites  int    `json:"detectedSites"`
	InstallStoreOK bool   `json:"installStoreOk"`
	PHPSocketFound bool   `json:"phpSocketFound"`
}

// Collect gathers a full VPS status snapshot. It is resilient: partial failures are surfaced in warnings.
func (s *Service) Collect(ctx context.Context) Snapshot {
	start := time.Now()
	snapshot := Snapshot{
		Status:    "ok",
		Timestamp: start.UTC().Format(time.RFC3339),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		CPU: CPUStatus{
			Cores: runtime.NumCPU(),
		},
		WordPress: WordPressStatus{
			SitesRoot: s.sitesRoot,
		},
	}

	var warnings []string

	if host, err := os.Hostname(); err == nil {
		snapshot.Hostname = host
	} else {
		warnings = append(warnings, "hostname: "+err.Error())
	}

	if kernel, err := readKernel(); err == nil {
		snapshot.Kernel = kernel
	} else {
		warnings = append(warnings, "kernel: "+err.Error())
	}

	if up, err := readUptime(); err == nil {
		snapshot.Uptime = up
	} else {
		warnings = append(warnings, "uptime: "+err.Error())
	}

	if load, err := readLoad(); err == nil {
		snapshot.Load = load
	} else {
		warnings = append(warnings, "load: "+err.Error())
	}

	if cpuUsage, measured, err := readCPUUsage(150 * time.Millisecond); err == nil {
		snapshot.CPU.UsagePercent = cpuUsage
		snapshot.CPU.SampleWindow = "150ms"
		snapshot.CPU.UsageMeasured = measured
	} else {
		warnings = append(warnings, "cpu usage: "+err.Error())
	}

	if mem, err := readMemory(); err == nil {
		snapshot.Memory = mem
	} else {
		warnings = append(warnings, "memory: "+err.Error())
	}

	disks := make([]DiskStatus, 0, 3)
	for _, p := range []string{"/", "/var", s.sitesRoot} {
		if strings.TrimSpace(p) == "" {
			continue
		}
		d, err := readDisk(ctx, p)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("disk(%s): %v", p, err))
			continue
		}
		disks = append(disks, d)
	}
	snapshot.Disk = disks

	services := []string{
		"nginx",
		"mariadb",
		"mysql",
		"php8.3-fpm",
		"php8.2-fpm",
		"php-fpm",
		"panelx-control-plane.service",
		"panelx-node-agent.service",
	}
	for _, name := range services {
		status := readServiceStatus(ctx, name)
		// Keep either active services or PanelX-related services in output to avoid noise.
		if status.Active || strings.Contains(name, "panelx") || strings.Contains(name, "nginx") || strings.Contains(name, "mariadb") {
			snapshot.Services = append(snapshot.Services, status)
		}
	}

	ifaces, err := readNetworkInterfaces()
	if err != nil {
		warnings = append(warnings, "network: "+err.Error())
	} else {
		snapshot.Network = ifaces
	}

	siteCount, err := countSiteDirectories(s.sitesRoot)
	if err != nil {
		warnings = append(warnings, "sites: "+err.Error())
	} else {
		snapshot.WordPress.DetectedSites = siteCount
	}

	installStorePath := filepath.Join(s.sitesRoot, ".panelx", "installations.json")
	if info, err := os.Stat(installStorePath); err == nil && !info.IsDir() {
		snapshot.WordPress.InstallStoreOK = true
	}

	if s.phpSocketPath != "" {
		if _, err := os.Stat(s.phpSocketPath); err == nil {
			snapshot.WordPress.PHPSocketFound = true
		}
	}

	if len(warnings) > 0 {
		snapshot.Status = "degraded"
		snapshot.Warnings = warnings
	}

	snapshot.CollectedMs = time.Since(start).Milliseconds()
	return snapshot
}

func readKernel() (string, error) {
	raw, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

func readUptime() (UptimeStatus, error) {
	raw, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return UptimeStatus{}, err
	}
	parts := strings.Fields(string(raw))
	if len(parts) < 1 {
		return UptimeStatus{}, fmt.Errorf("unexpected /proc/uptime format")
	}
	seconds, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return UptimeStatus{}, err
	}
	return UptimeStatus{
		Seconds: seconds,
		Human:   humanizeDuration(seconds),
	}, nil
}

func readLoad() (LoadStatus, error) {
	raw, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return LoadStatus{}, err
	}
	parts := strings.Fields(string(raw))
	if len(parts) < 3 {
		return LoadStatus{}, fmt.Errorf("unexpected /proc/loadavg format")
	}
	one, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return LoadStatus{}, err
	}
	five, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return LoadStatus{}, err
	}
	fifteen, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return LoadStatus{}, err
	}
	return LoadStatus{One: one, Five: five, Fifteen: fifteen}, nil
}

func readCPUUsage(window time.Duration) (float64, bool, error) {
	idleA, totalA, err := readProcStatCPU()
	if err != nil {
		return 0, false, err
	}
	time.Sleep(window)
	idleB, totalB, err := readProcStatCPU()
	if err != nil {
		return 0, false, err
	}

	idleDelta := idleB - idleA
	totalDelta := totalB - totalA
	if totalDelta == 0 {
		return 0, false, nil
	}

	usage := (1.0 - (float64(idleDelta) / float64(totalDelta))) * 100.0
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return round2(usage), true, nil
}

func readProcStatCPU() (idle uint64, total uint64, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				return 0, 0, fmt.Errorf("unexpected cpu line format")
			}
			for i := 1; i < len(fields); i++ {
				v, convErr := strconv.ParseUint(fields[i], 10, 64)
				if convErr != nil {
					return 0, 0, convErr
				}
				total += v
				if i == 4 || i == 5 { // idle + iowait
					idle += v
				}
			}
			return idle, total, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, err
	}
	return 0, 0, fmt.Errorf("cpu line not found")
}

func readMemory() (MemoryStatus, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemoryStatus{}, err
	}
	defer f.Close()

	var memTotalKB, memAvailKB, swapTotalKB, swapFreeKB uint64

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		key, val, ok := parseMemInfoLine(line)
		if !ok {
			continue
		}
		switch key {
		case "MemTotal":
			memTotalKB = val
		case "MemAvailable":
			memAvailKB = val
		case "SwapTotal":
			swapTotalKB = val
		case "SwapFree":
			swapFreeKB = val
		}
	}
	if err := scanner.Err(); err != nil {
		return MemoryStatus{}, err
	}
	if memTotalKB == 0 {
		return MemoryStatus{}, fmt.Errorf("MemTotal not found")
	}

	total := memTotalKB * 1024
	available := memAvailKB * 1024
	used := uint64(0)
	if total > available {
		used = total - available
	}

	swapTotal := swapTotalKB * 1024
	swapFree := swapFreeKB * 1024
	swapUsed := uint64(0)
	if swapTotal > swapFree {
		swapUsed = swapTotal - swapFree
	}

	memPct := percent(used, total)
	swapPct := percent(swapUsed, swapTotal)

	return MemoryStatus{
		TotalBytes:       total,
		AvailableBytes:   available,
		UsedBytes:        used,
		UsedPercent:      memPct,
		SwapTotalBytes:   swapTotal,
		SwapFreeBytes:    swapFree,
		SwapUsedBytes:    swapUsed,
		SwapUsedPercent:  swapPct,
		MeasurementUnits: "bytes",
	}, nil
}

func parseMemInfoLine(line string) (key string, valueKB uint64, ok bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", 0, false
	}
	key = strings.TrimSpace(parts[0])
	fields := strings.Fields(strings.TrimSpace(parts[1]))
	if len(fields) < 1 {
		return "", 0, false
	}
	v, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return key, v, true
}

func readDisk(ctx context.Context, path string) (DiskStatus, error) {
	cmd := exec.CommandContext(ctx, "df", "-B1", "--output=source,size,used,avail,pcent,target", path)
	out, err := cmd.Output()
	if err != nil {
		return DiskStatus{}, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return DiskStatus{}, fmt.Errorf("unexpected df output")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 6 {
		return DiskStatus{}, fmt.Errorf("unexpected df data row")
	}

	total, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return DiskStatus{}, err
	}
	used, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		return DiskStatus{}, err
	}
	free, err := strconv.ParseUint(fields[3], 10, 64)
	if err != nil {
		return DiskStatus{}, err
	}

	pctRaw := strings.TrimSuffix(fields[4], "%")
	pct, err := strconv.ParseFloat(pctRaw, 64)
	if err != nil {
		pct = percent(used, total)
	}

	return DiskStatus{
		Path:        fields[5],
		Filesystem:  fields[0],
		TotalBytes:  total,
		UsedBytes:   used,
		FreeBytes:   free,
		UsedPercent: round2(pct),
	}, nil
}

func readServiceStatus(ctx context.Context, name string) ManagedServiceStatus {
	status := ManagedServiceStatus{Name: name, State: "unknown"}

	cmd := exec.CommandContext(ctx, "systemctl", "is-active", name)
	out, err := cmd.CombinedOutput()
	state := strings.TrimSpace(string(out))

	if state == "" {
		state = "unknown"
	}
	status.State = state

	switch state {
	case "active":
		status.Active = true
		status.Description = "running"
	case "inactive":
		status.Active = false
		status.Description = "installed but not running"
	case "failed":
		status.Active = false
		status.Description = "service failed"
	case "activating":
		status.Active = false
		status.Description = "starting"
	case "deactivating":
		status.Active = false
		status.Description = "stopping"
	default:
		status.Active = false
		if err != nil {
			status.Description = strings.TrimSpace(err.Error())
		} else {
			status.Description = "not installed or unknown"
		}
	}

	return status
}

func readNetworkInterfaces() ([]NetworkInterfaceStatus, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []NetworkInterfaceStatus
	scanner := bufio.NewScanner(f)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		if lineNo <= 2 {
			continue // header
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		iface := strings.TrimSpace(parts[0])
		if iface == "lo" {
			continue
		}

		fields := strings.Fields(strings.TrimSpace(parts[1]))
		if len(fields) < 11 {
			continue
		}

		rxBytes, err1 := strconv.ParseUint(fields[0], 10, 64)
		rxErrs, err2 := strconv.ParseUint(fields[2], 10, 64)
		txBytes, err3 := strconv.ParseUint(fields[8], 10, 64)
		txErrs, err4 := strconv.ParseUint(fields[10], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			continue
		}

		out = append(out, NetworkInterfaceStatus{
			Name:           iface,
			RxBytes:        rxBytes,
			TxBytes:        txBytes,
			ReceiveErrors:  rxErrs,
			TransmitErrors: txErrs,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func countSiteDirectories(sitesRoot string) (int, error) {
	if strings.TrimSpace(sitesRoot) == "" {
		return 0, nil
	}

	entries, err := os.ReadDir(sitesRoot)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		count++
	}
	return count, nil
}

func humanizeDuration(seconds float64) string {
	total := int64(seconds)
	days := total / 86400
	total %= 86400
	hours := total / 3600
	total %= 3600
	minutes := total / 60
	secs := total % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, secs)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, secs)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return round2((float64(used) / float64(total)) * 100)
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
