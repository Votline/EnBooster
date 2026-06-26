// Package services contains interface that must be
// implemented by the all services.
// Also contains call methods with retry circuit breaker.
package services

import tele "gopkg.in/telebot.v3"

type Service interface {
	RegisterRoutes(bot *tele.Bot) error
	Close() error
}
