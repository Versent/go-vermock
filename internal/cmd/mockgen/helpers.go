package mockgen

import (
	"log"
	"strings"
)

func logErrors(log *log.Logger, errs ...error) {
	for _, err := range errs {
		log.Println(strings.Replace(err.Error(), "\n", "\n\t", -1))
	}
}
