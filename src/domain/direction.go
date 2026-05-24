package domain

import (
	"fmt"
	"strings"
)

type Direction string

const (
	DirectionUp   Direction = "up"
	DirectionDown Direction = "down"
)

func ParseDirection(value string) (Direction, error) {
	switch Direction(strings.ToLower(strings.TrimSpace(value))) {
	case DirectionUp:
		return DirectionUp, nil
	case DirectionDown:
		return DirectionDown, nil
	default:
		return "", fmt.Errorf("unknown direction %q: use up or down", value)
	}
}

func (d Direction) String() string {
	return string(d)
}

func (d Direction) Sign() string {
	if d == DirectionDown {
		return "-"
	}

	return "+"
}
