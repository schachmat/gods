package main

import "bufio"
import "fmt"
import "io/ioutil"
import "os"
import "os/exec"
import "strings"
import "time"

var cores = 1
var rxOld = 0
var txOld = 0

// init reads the count of cpu cores since it should not change during runtime
func init() {
	// You probably do not have hot-pluggable cpu cores
	if cpuinfo, err := ioutil.ReadFile("/proc/cpuinfo"); err == nil {
		cores = strings.Count(string(cpuinfo), "model name")
	}
}

// fixed builds a fixed width string with given pre- and fitting suffix
func fixed(pre string, rate int) string {
	if rate < 0 {
		return pre + " ERR"
	}

	var spd = float32(rate)
	var suf = "á" // default: display as B/s
	switch {
	case spd >= (1000 * 1024 * 1024): // > 999 MiB/s
		return "" + pre + "ERR"
	case spd >= (1000 * 1024): // display as MiB/s
		spd /= (1024 * 1024)
		suf = "ã"
		pre = "" + pre + ""
	case spd >= 1000: // display as KiB/s
		spd /= 1024
		suf = "â"
	}

	var formated = ""
	if spd >= 100 {
		formated = fmt.Sprintf("%3.0f", spd)
	} else if spd >= 10 {
		formated = fmt.Sprintf("%4.1f", spd)
	} else {
		formated = fmt.Sprintf(" %3.1f", spd)
	}
	return pre + strings.Replace(formated, ".", "à", 1) + suf
}

// updateNetSpeed reads current transfer rates of certain network interfaces
func updateNetSpeed() string {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return "Ð ERR Ñ ERR"
	}
	defer file.Close()

	var void = 0 // target for unused values
	var dev, rx, tx, rxNow, txNow = "", 0, 0, 0, 0
	var scanner = bufio.NewScanner(file)
	for scanner.Scan() {
		_, err = fmt.Sscanf(scanner.Text(), "%s %d %d %d %d %d %d %d %d %d",
			&dev, &rx, &void, &void, &void, &void, &void, &void, &void, &tx)
		switch dev { // ignore devices like tun, tap, lo, ...
		case "eth0:", "eth1:", "wlan0:", "ppp:":
			rxNow += rx
			txNow += tx
		}
	}

	defer func() { rxOld, txOld = rxNow, txNow }()
	return fmt.Sprintf("%s %s", fixed("Ð", rxNow-rxOld), fixed("Ñ", txNow-txOld))
}

// colored surrounds the percentage with color escapes if it is >= 70
func colored(icon string, percentage int) string {
	if percentage >= 100 {
		return fmt.Sprintf("%s%3d", icon, percentage)
	} else if percentage >= 70 {
		return fmt.Sprintf("%s%3d", icon, percentage)
	}
	return fmt.Sprintf("%s%3d", icon, percentage)
}

// updateCpuUse reads the last minute sysload and scales it to the core count
func updateCpuUse() string {
	var load float32
	var loadavg, err = ioutil.ReadFile("/proc/loadavg")
	if err != nil {
		return "ÏERR"
	}
	_, err = fmt.Sscanf(string(loadavg), "%f", &load)
	if err != nil {
		return "ÏERR"
	}
	return colored("Ï", int(load*100.0/float32(cores)))
}

// updateMemUse reads the memory used by applications and scales to [0, 100]
func updateMemUse() string {
	var file, err = os.Open("/proc/meminfo")
	if err != nil {
		return "ÞERR"
	}
	defer file.Close()

	// done must equal the flag combination (0001 | 0010 | 0100 | 1000) = 15
	var total, used, done = 0, 0, 0
	for info := bufio.NewScanner(file); done != 15 && info.Scan(); {
		var prop, val = "", 0
		if _, err = fmt.Sscanf(info.Text(), "%s %d", &prop, &val); err != nil {
			return "ÞERR"
		}
		switch prop {
		case "MemTotal:":
			total = val
			used += val
			done |= 1
		case "MemFree:":
			used -= val
			done |= 2
		case "Buffers:":
			used -= val
			done |= 4
		case "Cached:":
			used -= val
			done |= 8
		}
	}
	return colored("Þ", used*100/total)
}

// main updates the dwm statusbar every second
func main() {
	// sleep until beginning of next second
	var now = time.Now()
	time.Sleep(now.Truncate(time.Second).Add(time.Second).Sub(now))

	// update status every full second
	for clock := range time.Tick(time.Second) {
		var elements = []string{""}
		elements = append(elements, updateNetSpeed())
		elements = append(elements, updateCpuUse())
		elements = append(elements, updateMemUse())
		elements = append(elements, clock.Format("Mon 02 Ý 15:04:05"))
		exec.Command("xsetroot", "-name", strings.Join(elements, "û")).Run()
	}
}
