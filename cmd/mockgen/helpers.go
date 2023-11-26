package main

import (
	"log"
	"strings"
)

func logErrors(errs ...error) {
	for _, err := range errs {
		log.Println(strings.Replace(err.Error(), "\n", "\n\t", -1))
	}
}
