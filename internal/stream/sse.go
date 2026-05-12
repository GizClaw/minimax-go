package stream

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

// Event represents a single SSE event.
type Event struct {
	ID    string
	Event string
	Data  string
	Retry int
}

// Reader reads SSE events one by one.
type Reader struct {
	reader *bufio.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{reader: bufio.NewReader(r)}
}

// Next returns the next complete event, or io.EOF when exhausted.
func (r *Reader) Next() (Event, error) {
	var (
		event     Event
		dataLines []string
		hasField  bool
	)

	for {
		line, isEOF, err := r.readLine()
		if err != nil {
			return Event{}, err
		}

		if isEOF && line == "" {
			return finalizeEvent(event, dataLines, hasField)
		}

		line = strings.TrimRight(line, "\r\n")
		if isEOF {
			if line == "" {
				return finalizeEvent(event, dataLines, hasField)
			}
			if strings.HasPrefix(line, ":") {
				if !hasField {
					return Event{}, io.EOF
				}
				return finalizeEvent(event, dataLines, hasField)
			}
		}

		if line == "" {
			if hasField {
				return finalizeEvent(event, dataLines, hasField)
			}
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value := splitField(line)
		hasField = true
		applyEventField(&event, &dataLines, field, value)

		if isEOF {
			return finalizeEvent(event, dataLines, hasField)
		}
	}
}

func (r *Reader) readLine() (string, bool, error) {
	line, err := r.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", false, err
	}

	return line, errors.Is(err, io.EOF), nil
}

func finalizeEvent(event Event, dataLines []string, hasField bool) (Event, error) {
	if !hasField {
		return Event{}, io.EOF
	}

	event.Data = strings.Join(dataLines, "\n")
	return event, nil
}

func applyEventField(event *Event, dataLines *[]string, field, value string) {
	if event == nil || dataLines == nil {
		return
	}

	switch field {
	case "id":
		event.ID = value
	case "event":
		event.Event = value
	case "data":
		*dataLines = append(*dataLines, value)
	case "retry":
		retry, convErr := strconv.Atoi(value)
		if convErr == nil && retry >= 0 {
			event.Retry = retry
		}
	}
}

func splitField(line string) (string, string) {
	before, after, ok := strings.Cut(line, ":")
	if !ok {
		return line, ""
	}

	field := before
	value := after
	if strings.HasPrefix(value, " ") {
		value = value[1:]
	}

	return field, value
}
