package releasehandler

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"mime/multipart"
)

// getSingleFormValue - returns the only value of the form or generates error
func getSingleFormValue(form *multipart.Form, key string) (error, string) {
	values, ok := form.Value[key]

	if !ok {
		return fmt.Errorf("provide `%s` form value", key), ""
	}

	log.Debugf("[release] Got values for key `%s`: %s", key, values)

	if len(values) != 1 {
		return fmt.Errorf("provide single `%s` form value", key), ""
	}

	return nil, values[0]
}
