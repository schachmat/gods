// This programm collects some system information, formats it nicely and sets
// the X root windows name so it can be displayed in the dwm status bar.
//
// The strange characters in the output are used by dwm to colorize the output
// ( to , needs the http://dwm.suckless.org/patches/statuscolors patch) and
// as Icons or separators (e.g. "Ý"). If you don't use the status-18 font
// (https://github.com/schachmat/status-18), you should probably exchange them
// by something else ("CPU", "MEM", "|" for separators, …).
//
// For license information see the file LICENSE
package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	bpsSign   = "á"
	kibpsSign = "â"
	mibpsSign = "ã"

	unpluggedSign = "è"
	pluggedSign   = "é"

	cpuSign = "Ï"
	memSign = "Þ"

	netReceivedSign    = "Ð"
	netTransmittedSign = "Ñ"

	floatSeparator = "à"
	dateSeparator  = "Ý"
	fieldSeparator = "û"
)

var (
	netDevs = map[string]struct{}{
		"eth0:": {},
		"eth1:": {},
		"wlan0:": {},
		"ppp0:": {},
	}
	cores = runtime.NumCPU() // count of cores to scale cpu usage
	rxOld = 0
	txOld = 0
)

// fixed builds a fixed width string with given pre- and fitting suffix
func fixed(pre string, rate int) string {
	if rate < 0 {
		return pre + " ERR"
	}

	var decDigit = 0
	var suf = bpsSign // default: display as B/s

	switch {
	case rate >= (1000 * 1024 * 1024): // > 999 MiB/s
		return "" + pre + "ERR"
	case rate >= (1000 * 1024): // display as MiB/s
		decDigit = (rate / 1024 / 102) % 10
		rate /= (1024 * 1024)
		suf = mibpsSign
		pre = "" + pre + ""
	case rate >= 1000: // display as KiB/s
		decDigit = (rate / 102) % 10
		rate /= 1024
		suf = kibpsSign
	}

	var formated = ""
	if rate >= 100 {
		formated = fmt.Sprintf("%3d", rate)
	} else if rate >= 10 {
		formated = fmt.Sprintf("%2d.%1d", rate, decDigit)
	} else {
		formated = fmt.Sprintf(" %1d.%1d", rate, decDigit)
	}
	return pre + strings.Replace(formated, ".", floatSeparator, 1) + suf
}

// updateNetUse reads current transfer rates of certain network interfaces
func updateNetUse() string {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return netReceivedSign + " ERR " + netTransmittedSign + " ERR"
	}
	defer file.Close()

	var void = 0 // target for unused values
	var dev, rx, tx, rxNow, txNow = "", 0, 0, 0, 0
	var scanner = bufio.NewScanner(file)
	for scanner.Scan() {
		_, err = fmt.Sscanf(scanner.Text(), "%s %d %d %d %d %d %d %d %d %d",
			&dev, &rx, &void, &void, &void, &void, &void, &void, &void, &tx)
		if _, ok := netDevs[dev]; ok {
			rxNow += rx
			txNow += tx
		}
	}

	defer func() { rxOld, txOld = rxNow, txNow }()
	return fmt.Sprintf("%s %s", fixed(netReceivedSign, rxNow-rxOld), fixed(netTransmittedSign, txNow-txOld))
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

// updatePower reads the current battery and power plug status
func updatePower() string {
	const powerSupply = "/sys/class/power_supply/"
	var enFull, enNow, enPerc int = 0, 0, 0
	var plugged, err = ioutil.ReadFile(powerSupply + "AC/online")
	if err != nil {
		return "ÏERR"
	}
	batts, err := ioutil.ReadDir(powerSupply)
	if err != nil {
		return "ÏERR"
	}

	readval := func(name, field string) int {
		var path = powerSupply + name + "/"
		var file []byte
		if tmp, err := ioutil.ReadFile(path + "energy_" + field); err == nil {
			file = tmp
		} else if tmp, err := ioutil.ReadFile(path + "charge_" + field); err == nil {
			file = tmp
		} else {
			return 0
		}

		if ret, err := strconv.Atoi(strings.TrimSpace(string(file))); err == nil {
			return ret
		}
		return 0
	}

	for _, batt := range batts {
		name := batt.Name()
		if !strings.HasPrefix(name, "BAT") {
			continue
		}

		enFull += readval(name, "full")
		enNow += readval(name, "now")
	}

	if enFull == 0 { // Battery found but no readable full file.
		return "ÏERR"
	}

	enPerc = enNow * 100 / enFull
	var icon = unpluggedSign
	if string(plugged) == "1\n" {
		icon = pluggedSign
	}

	if enPerc <= 5 {
		return fmt.Sprintf("%s%3d", icon, enPerc)
	} else if enPerc <= 10 {
		return fmt.Sprintf("%s%3d", icon, enPerc)
	}
	return fmt.Sprintf("%s%3d", icon, enPerc)
}

// updateCPUUse reads the last minute sysload and scales it to the core count
func updateCPUUse() string {
	var load float32
	var loadavg, err = ioutil.ReadFile("/proc/loadavg")
	if err != nil {
		return cpuSign + "ERR"
	}
	_, err = fmt.Sscanf(string(loadavg), "%f", &load)
	if err != nil {
		return cpuSign + "ERR"
	}
	return colored(cpuSign, int(load*100.0/float32(cores)))
}

// updateMemUse reads the memory used by applications and scales to [0, 100]
func updateMemUse() string {
	var file, err = os.Open("/proc/meminfo")
	if err != nil {
		return memSign + "ERR"
	}
	defer file.Close()

	// done must equal the flag combination (0001 | 0010 | 0100 | 1000) = 15
	var total, used, done = 0, 0, 0
	for info := bufio.NewScanner(file); done != 15 && info.Scan(); {
		var prop, val = "", 0
		if _, err = fmt.Sscanf(info.Text(), "%s %d", &prop, &val); err != nil {
			return memSign + "ERR"
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
	return colored(memSign, used*100/total)
}

// main updates the dwm statusbar every second
func main() {
	for {
		var status = []string{
			"",
			updateNetUse(),
			updateCPUUse(),
			updateMemUse(),
			updatePower(),
			time.Now().Local().Format("Mon 02 " + dateSeparator + " 15:04:05"),
		}
		exec.Command("xsetroot", "-name", strings.Join(status, fieldSeparator)).Run()

		// sleep until beginning of next second
		var now = time.Now()
		time.Sleep(now.Truncate(time.Second).Add(time.Second).Sub(now))
	}
}
