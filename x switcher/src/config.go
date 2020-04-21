package main

import (
	"fmt"
	"github.com/gvalkov/golang-evdev"
	"github.com/BurntSushi/toml"		// TOML
	flag "github.com/spf13/pflag"       // CLI keys like python's "argparse". More flexible, comparing with "gnuflag"
	"io/ioutil"
	"os"
	"regexp"
//	"strconv"
)

const (
	DAEMON_NAME = "xswitcher"
	CONFIG_PATH = "~/.config/xswitcher/xswitcher.conf"
)

// Config in TOML
type ActionKeys struct {
	Add []string  `toml:"add,omitempty"`
	Drop []string `toml:"drop,omitempty"`
	Test []string `toml:"test,omitempty"`
}

type Config struct {
	ActionKeys ActionKeys
}

type actionFunc func(event t_key)

var (
//    CONF *Config
    DEBUG *bool
    VERBOSE *bool

	add actionFunc = Add
	test actionFunc = TestSeq
	drop actionFunc = Drop

    ACTIONS [768]actionFunc
	KEY_RANGE = regexp.MustCompile(`^[\[\]A-Z0-9_=.,;/\\-]+\.\.[\[\]A-Z0-9_=.,;/\\-]+$`)
	KEY_SPLIT = regexp.MustCompile(`\.\.`)
)

func parseKeys(keys []string, action actionFunc, errMsg string) {
	for _, key := range keys {
		if k, ok := key_def[key]; ok {
			ACTIONS[k] = action
		} else {
			if KEY_RANGE.MatchString(key) {
				k1 := uint(0)
				k2 := uint(0)
				kk := KEY_SPLIT.Split(key, 2)
				if k1, ok = key_def[ kk[0] ]; !ok {
					panic(fmt.Sprintf("Invalid key for %s: %s", errMsg, key))
				}
				if k2, ok = key_def[ kk[1] ]; !ok {
					panic(fmt.Sprintf("Invalid key for %s: %s", errMsg, key))
				}

				for k := k1; k <= k2 ; k++ {
					ACTIONS[k] = action
				}
			} else {
					panic(fmt.Sprintf("Invalid key for %s: %s", errMsg, key))
			}
		}

	}
}

func parseConfigFile() {
	for i := 0; i < 768; i++ { // Fill default ACTIONS for each keyboard code
		switch key := i; {
			case key == evdev.KEY_BREAK || key == evdev.KEY_PAUSE:
				ACTIONS[i] = add
			case key < evdev.KEY_1: // drop
				ACTIONS[i] = drop
			case key == evdev.KEY_MINUS: // drop
				ACTIONS[i] = drop
			case key < evdev.KEY_BACKSPACE: // pass
				ACTIONS[i] = add
			case key == evdev.KEY_BACKSPACE: // pass !!! but don't count as char
				ACTIONS[i] = add
			case key < evdev.KEY_Q: // drop
				ACTIONS[i] = drop
			case key < evdev.KEY_ENTER: // pass
				ACTIONS[i] = add
			case key < evdev.KEY_LEFTCTRL: // drop
				ACTIONS[i] = drop
			case key == evdev.KEY_LEFTCTRL: // CTRL
				ACTIONS[i] = add
			case key <= evdev.KEY_LEFTSHIFT: // pass
				ACTIONS[i] = add
			case key <= evdev.KEY_RIGHTSHIFT: // pass
				ACTIONS[i] = add
			case key == evdev.KEY_KPASTERISK: // pass keypad
				ACTIONS[i] = add
			case key == evdev.KEY_LEFTALT: // CTRL
				ACTIONS[i] = add
			case key == evdev.KEY_SPACE: // pass
				ACTIONS[i] = add
			case key == evdev.KEY_CAPSLOCK: // pass
				ACTIONS[i] = add
			case key <= evdev.KEY_F10: // F1..F10 ignore
				ACTIONS[i] = test
			case key == evdev.KEY_F11: // F11 ignore
				ACTIONS[i] = test
			case key == evdev.KEY_F12: // F12 ignore
				ACTIONS[i] = test
			case key <= evdev.KEY_SCROLLLOCK: // pass
				ACTIONS[i] = add
			case key < evdev.KEY_ZENKAKUHANKAKU: // pass keypad
				ACTIONS[i] = add
			case key == evdev.KEY_KPCOMMA: // pass keypad
				ACTIONS[i] = add
			case key == evdev.KEY_KPLEFTPAREN: // pass keypad
				ACTIONS[i] = add
			case key == evdev.KEY_KPRIGHTPAREN: // pass keypad
				ACTIONS[i] = add
			case key == evdev.KEY_RIGHTCTRL: // CTRL
				ACTIONS[i] = add
			case key == evdev.KEY_KPSLASH: // pass
				ACTIONS[i] = add
			case key == evdev.KEY_RIGHTALT: // CTRL
				ACTIONS[i] = add
			case key == evdev.KEY_LEFTMETA: // ???
				ACTIONS[i] = add
			case key == evdev.KEY_RIGHTMETA: // ???
				ACTIONS[i] = add
			default: // test
				ACTIONS[i] = drop
		}
	}

    config_path_ := CONFIG_PATH
    config_path := &config_path_

	conf := &Config{}

	if env_config, ok := os.LookupEnv("CONFIG"); ok {
		*config_path = env_config
	}

    F := flag.NewFlagSet("", flag.ContinueOnError)
    config_path = F.StringP("conf", "c", *config_path, "Non-default config location")
    DEBUG = F.BoolP("debug", "d", *DEBUG, "Debug log level")
    VERBOSE = F.BoolP("verbose", "v", false, "Increase log level to NOTICE")
	F.Init("", flag.ExitOnError)
    F.Parse(os.Args[1:])

	conf_file, err := os.Open(*config_path)
	if err != nil {
		fmt.Println(fmt.Errorf("Config error: unable to open config file: %s", err.Error()))
		fmt.Println("* Using defaults!")
		return
	}
	defer conf_file.Close()

	conf_, err := ioutil.ReadAll(conf_file)
	if err != nil {
		fmt.Println(fmt.Errorf("Config error: unable to read config file: %s", err.Error()))
		fmt.Println("* Using defaults!")
		return
	}

/* Yes! Now BurntSushi.TOML has usable tree-view, like JSON in execd.jsonCommand()!
   So, I can manage parse process on-the-fly, involving extra syntax
   (but still limited by TOML validator).
*/
//	var result map [string]interface{}
//    toml.Unmarshal(conf_, &result)
//    fmt.Println(result["WindowClasses"])

	if err := toml.Unmarshal(conf_, &conf); err != nil {
		panic(fmt.Errorf("Config error: unable to parse config file: %s", err.Error()))
	}

	parseKeys(conf.ActionKeys.Add, add, "Add")
	parseKeys(conf.ActionKeys.Drop, drop, "Drop")
	parseKeys(conf.ActionKeys.Test, test, "Test")

	return
}
