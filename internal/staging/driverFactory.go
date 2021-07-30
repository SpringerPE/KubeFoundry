package staging

import (
	"fmt"
	"sync"

	config "kubefoundry/internal/config"
	log "kubefoundry/internal/log"
)

var (
	muStagers sync.RWMutex
	stagers   = make(map[string]AppStaging)
)

// Drivers must call this function in their init
func RegisterStagingDriver(driver string, da AppStaging) {
	muStagers.Lock()
	defer muStagers.Unlock()

	if da == nil {
		panic("Driver is nil")
	}
	if _, exist := stagers[driver]; exist {
		panic("Driver already registered")
	}
	stagers[driver] = da
}

func LoadStagingDriver(driver string, c *config.Config, l log.Logger) (AppStaging, error) {
	muStagers.RLock()
	defer muStagers.RUnlock()

	if driver, exist := stagers[driver]; exist {
		stager, err := driver.New(c, l)
		if err == nil {
			return stager, nil
		} else {
			err = fmt.Errorf("Unable to initialize staging driver '%s': %s", driver, err.Error())
			l.Error(err)
			return nil, err
		}
	} else {
		err := fmt.Errorf("Unknown staging driver, not included in the program")
		l.Error(err)
		return nil, err
	}
}

func ListStaginDrivers() []string {
	muStagers.RLock()
	defer muStagers.RUnlock()

	result := []string{}
	for k, _ := range stagers {
		result = append(result, k)
	}
	return result
}
