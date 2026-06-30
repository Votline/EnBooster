// Package services contains interfaces for services
package services

type Closable interface {
	Close() error
	GetName() string
}
