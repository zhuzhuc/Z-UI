package server

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type SystemMetrics struct {
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryPercent float64 `json:"memoryPercent"`
	DiskPercent   float64 `json:"diskPercent"`
	MemoryUsedMB  uint64  `json:"memoryUsedMB"`
	MemoryTotalMB uint64  `json:"memoryTotalMB"`
	DiskUsedGB    uint64  `json:"diskUsedGB"`
	DiskTotalGB   uint64  `json:"diskTotalGB"`
	Platform      string  `json:"platform"`
}

func collectSystemMetrics() SystemMetrics {
	out := SystemMetrics{CPUPercent: -1, MemoryPercent: -1, DiskPercent: -1, Platform: runtime.GOOS}

	switch runtime.GOOS {
	case "linux":
		if cpu, ok := linuxCPUPercent(); ok {
			out.CPUPercent = cpu
		}
		if totalMB, usedMB, memPercent, ok := linuxMemoryPercent(); ok {
			out.MemoryTotalMB = totalMB
			out.MemoryUsedMB = usedMB
			out.MemoryPercent = memPercent
		}
	case "darwin":
		if cpu, ok := darwinCPUPercent(); ok {
			out.CPUPercent = cpu
		}
		if totalMB, usedMB, memPercent, ok := darwinMemoryPercent(); ok {
			out.MemoryTotalMB = totalMB
			out.MemoryUsedMB = usedMB
			out.MemoryPercent = memPercent
		}
	}
	if totalGB, usedGB, diskPercent, ok := diskPercent("/"); ok {
		out.DiskTotalGB = totalGB
		out.DiskUsedGB = usedGB
		out.DiskPercent = diskPercent
	}

	return out
}

func darwinCPUPercent() (float64, bool) {
	out, err := exec.Command("/bin/sh", "-lc", "ps -A -o %cpu").Output()
	if err != nil {
		return 0, false
	}
	lines := strings.Split(string(out), "\n")
	var sum float64
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "%CPU" {
			continue
		}
		v, err := strconv.ParseFloat(line, 64)
		if err != nil {
			continue
		}
		sum += v
	}
	cores := float64(runtime.NumCPU())
	if cores <= 0 {
		cores = 1
	}
	usage := sum / cores
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return usage, true
}

func darwinMemoryPercent() (totalMB uint64, usedMB uint64, percent float64, ok bool) {
	totalOut, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0, 0, 0, false
	}
	totalBytes, err := strconv.ParseUint(strings.TrimSpace(string(totalOut)), 10, 64)
	if err != nil || totalBytes == 0 {
		return 0, 0, 0, false
	}

	vmOut, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0, 0, 0, false
	}
	text := string(vmOut)
	pageSize := uint64(4096)
	if idx := strings.Index(text, "page size of "); idx >= 0 {
		rest := text[idx+len("page size of "):]
		end := strings.Index(rest, " bytes")
		if end > 0 {
			if p, err := strconv.ParseUint(strings.TrimSpace(rest[:end]), 10, 64); err == nil && p > 0 {
				pageSize = p
			}
		}
	}

	readPages := func(key string) uint64 {
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, key) {
				continue
			}
			parts := strings.Split(line, ":")
			if len(parts) != 2 {
				continue
			}
			n := strings.TrimSpace(parts[1])
			n = strings.TrimSuffix(n, ".")
			n = strings.ReplaceAll(n, ".", "")
			v, _ := strconv.ParseUint(n, 10, 64)
			return v
		}
		return 0
	}

	active := readPages("Pages active")
	wired := readPages("Pages wired down")
	compressed := readPages("Pages occupied by compressor")

	usedBytes := (active + wired + compressed) * pageSize
	if usedBytes > totalBytes {
		usedBytes = totalBytes
	}

	totalMB = totalBytes / (1024 * 1024)
	usedMB = usedBytes / (1024 * 1024)
	percent = float64(usedBytes) / float64(totalBytes) * 100
	return totalMB, usedMB, percent, true
}

func linuxCPUPercent() (float64, bool) {
	t1, i1, ok := readProcCPUStat()
	if !ok {
		return 0, false
	}
	time.Sleep(150 * time.Millisecond)
	t2, i2, ok := readProcCPUStat()
	if !ok || t2 <= t1 || i2 < i1 {
		return 0, false
	}
	dt := float64(t2 - t1)
	di := float64(i2 - i1)
	usage := (dt - di) / dt * 100
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return usage, true
}

func readProcCPUStat() (total uint64, idle uint64, ok bool) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	if !s.Scan() {
		return 0, 0, false
	}
	line := strings.Fields(s.Text())
	if len(line) < 8 || line[0] != "cpu" {
		return 0, 0, false
	}
	vals := make([]uint64, 0, len(line)-1)
	for _, one := range line[1:] {
		v, err := strconv.ParseUint(one, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		vals = append(vals, v)
		total += v
	}
	idle = vals[3]
	if len(vals) > 4 {
		idle += vals[4]
	}
	return total, idle, true
}

func linuxMemoryPercent() (totalMB uint64, usedMB uint64, percent float64, ok bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, false
	}
	defer f.Close()

	var totalKB, availableKB uint64
	s := bufio.NewScanner(f)
	for s.Scan() {
		parts := strings.Fields(s.Text())
		if len(parts) < 2 {
			continue
		}
		switch parts[0] {
		case "MemTotal:":
			v, _ := strconv.ParseUint(parts[1], 10, 64)
			totalKB = v
		case "MemAvailable:":
			v, _ := strconv.ParseUint(parts[1], 10, 64)
			availableKB = v
		}
	}
	if totalKB == 0 {
		return 0, 0, 0, false
	}
	usedKB := totalKB - availableKB
	totalMB = totalKB / 1024
	usedMB = usedKB / 1024
	percent = float64(usedKB) / float64(totalKB) * 100
	return totalMB, usedMB, percent, true
}

func diskPercent(path string) (totalGB uint64, usedGB uint64, percent float64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, 0, false
	}
	total := st.Blocks * uint64(st.Bsize)
	avail := st.Bavail * uint64(st.Bsize)
	used := total - avail
	if total == 0 {
		return 0, 0, 0, false
	}
	totalGB = total / (1024 * 1024 * 1024)
	usedGB = used / (1024 * 1024 * 1024)
	percent = float64(used) / float64(total) * 100
	return totalGB, usedGB, percent, true
}
