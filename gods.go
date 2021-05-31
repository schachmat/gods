// Command gods collects some system information, formats it nicely and sets
// the X root windows name so it can be displayed in the dwm status bar.
//
// The low value runes in the output are used by dwm to colorize the output
// (\u0001 to \u0006, needs the http://dwm.suckless.org/patches/statuscolors
// patch) and as Icons or separators (e.g. "\uf246"). This setup is recommended
// for using the following fonts in dwm config.h: primary: dejavu sans mono,
// fallback: material design icons.
//
// For license information see the file LICENSE
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	iconCPU           = "\uf35b"
	iconDateTime      = "\uf150"
	iconMemory        = "\uf193"
	iconNetRX         = "\uf046"
	iconNetTX         = "\uf05e"
	iconPowerBattery  = "\uf080"
	iconPowerCharging = "\uf084"
	iconVolume        = "\uf57e"
	iconVolumeMuted   = "\uf581"

	fieldSeparator = " "
)

const (
	reset  = 0
	green  = 1
	yellow = 2
	red    = 3
)

var (
	color   = []string{"\u0001", "\u0002", "\u0003", "\u0006"}
	ignoreNetDevPrefix = []string{"lo", "tun", "tap"}
	cores = runtime.NumCPU() // count of cores to scale cpu usage
	rxOld = 0
	txOld = 0
	unmutedLine = regexp.MustCompile("^[[:blank:]]*Mute: no$")
	volumeLine = regexp.MustCompile("^[[:blank:]]*Volume: ")
	channelVolume = regexp.MustCompile("[[:digit:]]+%")
)

// fixed builds a fixed width string with given icon and fitting suffix
func fixed(icon string, rate int) string {
	if rate < 0 {
		return color[red] + " ERR" + color[reset] + icon
	}

	var decDigit = 0
	var suf = "B" // default: display as B/s

	switch {
	case rate >= (1000 * 1024 * 1024): // > 999 MiB/s
		return color[red] + " ERR" + color[reset] + icon
	case rate >= (1000 * 1024): // display as MiB/s
		decDigit = (rate / 1024 / 102) % 10
		rate /= (1024 * 1024)
		suf = "M"
		icon = color[green] + icon + color[reset]
	case rate >= 1000: // display as KiB/s
		decDigit = (rate / 102) % 10
		rate /= 1024
		suf = "K"
	}

	if rate >= 100 {
		return fmt.Sprintf("%s%3d%s%s", color[reset], rate, suf, icon)
	} else if rate >= 10 {
		return fmt.Sprintf("%s %2d%s%s", color[reset], rate, suf, icon)
	} else {
		return fmt.Sprintf("%s%1d.%1d%s%s", color[reset], rate, decDigit, suf, icon)
	}
}

// updateNetUse reads current transfer rates of certain network interfaces
func updateNetUse() string {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		e := " " + color[red] + "ERR" + color[reset]
		return e + iconNetRX + " " + e + iconNetTX
	}
	defer file.Close()

	var void = 0 // target for unused values
	var dev, rx, tx, rxNow, txNow = "", 0, 0, 0, 0
	var scanner = bufio.NewScanner(file)
	// Skip first two lines (table header)
	scanner.Scan()
	scanner.Scan()
	for scanner.Scan() {
		_, err = fmt.Sscanf(scanner.Text(), "%s %d %d %d %d %d %d %d %d %d",
			&dev, &rx, &void, &void, &void, &void, &void, &void, &void, &tx)
		for _, ignorePrefix := range ignoreNetDevPrefix {
			if strings.HasPrefix(dev, ignorePrefix) {
				continue
			}
		}
		rxNow += rx
		txNow += tx
	}

	defer func() { rxOld, txOld = rxNow, txNow }()
	return fmt.Sprintf(
		"%s %s",
		fixed(iconNetRX, rxNow-rxOld),
		fixed(iconNetTX, txNow-txOld),
	)
}

// colored surrounds the percentage with color escapes if it is outside of a
// formatable range or urgent is true or warn is true.
func colored(icon string, percentage int, urgent, warn bool) string {
	if percentage >= 1000 {
		return fmt.Sprintf(" %sHI%s%s", color[red], color[reset], icon)
	} else if percentage < 0 {
		return fmt.Sprintf("%sNEG%s%s", color[red], color[reset], icon)
	} else if urgent {
		return fmt.Sprintf("%3d%s%s", percentage, color[red], icon)
	} else if warn {
		return fmt.Sprintf("%3d%s%s", percentage, color[yellow], icon)
	}
	return fmt.Sprintf("%3d%s", percentage, icon)
}

// updatePower reads the current battery and power plug status
func updatePower() string {
	const powerSupply = "/sys/class/power_supply/"
	var enFull, enNow int = 0, 0
	var plugged, err = ioutil.ReadFile(powerSupply + "AC/online")
	if err != nil {
		return color[red] + "ERR" + color[reset] + iconPowerBattery
	}
	batts, err := ioutil.ReadDir(powerSupply)
	if err != nil {
		return color[red] + "ERR" + color[reset] + iconPowerBattery
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
		return color[red] + "ERR" + color[reset] + iconPowerBattery
	}

	p := enNow * 100 / enFull
	var icon = iconPowerBattery
	if string(plugged) == "1\n" {
		icon = iconPowerCharging
	}

	return colored(icon, p, p<=10, p<=20)
}

// updateVolume reads the volume from pulseaudio
func updateVolume() string {
	cmd := exec.Command("pactl", "list", "sinks")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		fmt.Println(err)
		return color[red] + "ERR" + color[reset] + iconVolume
	}
	scanner := bufio.NewScanner(bytes.NewBuffer(out))
	chanCount := 0
	volSum := 0
	icon := iconVolumeMuted
	for scanner.Scan() {
		line := scanner.Text()
		if unmutedLine.MatchString(line) {
			icon = iconVolume
		}
		if !volumeLine.MatchString(line) {
			continue
		}
		m := channelVolume.FindAllString(line, -1)
		for _, c := range m {
			var v int
			if _, err := fmt.Sscanf(c, "%d%%", &v); err == nil {
				chanCount++
				volSum += v
			}
		}
	}
	if err := scanner.Err(); err != nil || chanCount == 0 {
		return color[red] + "ERR" + color[reset] + iconVolume
	}

	p := volSum/chanCount
	return colored(icon, p, p>100, p>=90)
}

// updateCPUUse reads the last minute sysload and scales it to the core count
func updateCPUUse() string {
	var load float32
	var loadavg, err = ioutil.ReadFile("/proc/loadavg")
	if err != nil {
		return color[red] + "ERR" + color[reset] + iconCPU
	}
	_, err = fmt.Sscanf(string(loadavg), "%f", &load)
	if err != nil {
		return color[red] + "ERR" + color[reset] + iconCPU
	}
	p := int(load*100.0/float32(cores))
	return colored(iconCPU, p, p>=100, p>=70)
}

// updateMemUse reads the memory used by applications and scales to [0, 100]
func updateMemUse() string {
	var file, err = os.Open("/proc/meminfo")
	if err != nil {
		return color[red] + "ERR" + color[reset] + iconMemory
	}
	defer file.Close()

	// done must equal the flag combination (0001 | 0010 | 0100 | 1000) = 15
	var total, used, done = 0, 0, 0
	for info := bufio.NewScanner(file); done != 15 && info.Scan(); {
		var prop, val = "", 0
		if _, err = fmt.Sscanf(info.Text(), "%s %d", &prop, &val); err != nil {
			return color[red] + "ERR" + color[reset] + iconMemory
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
	p := used*100/total
	return colored(iconMemory, p, p>=95, p>=70)
}

// main updates the dwm statusbar every second
func main() {
	for {
		status := []string{
			"",
			updateNetUse(),
			updateCPUUse(),
			updateMemUse(),
			updatePower(),
			updateVolume(),
			time.Now().Local().Format("Mon 02 " + iconDateTime + " 15:04:05"),
		}
		s := strings.Join(status, color[reset]+fieldSeparator)
		exec.Command("xsetroot", "-name", s).Run()

		// sleep until beginning of next second
		var now = time.Now()
		time.Sleep(now.Truncate(time.Second).Add(time.Second).Sub(now))
	}
}
