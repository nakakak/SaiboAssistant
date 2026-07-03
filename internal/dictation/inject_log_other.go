//go:build windows || linux

package dictation

import "log"

func LogInjectCapability(injectEnabled bool) {
	if !injectEnabled {
		log.Println("dictation: inject disabled in config")
		return
	}
	log.Println("dictation: inject enabled")
}
