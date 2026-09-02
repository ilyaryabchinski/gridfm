package operations

import (
	"context"
	"fmt"
)

// conflictCoordinator serializes conflict questions for the running job,
// honoring an apply-to-all decision once the user makes one.
type conflictCoordinator struct {
	publish func(context.Context, Event) error

	stored    Answer
	storedAll bool
}

// Resolve blocks until the user answers the conflict for target, the job
// is cancelled, or an apply-to-all decision from an earlier answer applies.
func (c *conflictCoordinator) Resolve(ctx context.Context, target string) (Answer, error) {
	if c.storedAll {
		return c.stored, nil
	}

	answerCh := make(chan Answer, 1)
	pubErr := c.publish(ctx, QuestionEvent{Target: target, AnswerCh: answerCh})
	if pubErr != nil {
		return Answer{Action: ConflictSkip}, fmt.Errorf("conflict resolution interrupted: %w", pubErr)
	}

	select {
	case answer := <-answerCh:
		if answer.ApplyAll && answer.Action != ConflictAbort {
			c.stored = answer
			c.storedAll = true
		}

		return answer, nil
	case <-ctx.Done():
		err := ctx.Err()
		if err != nil {
			err = fmt.Errorf("conflict resolution interrupted: %w", err)
		}

		return Answer{Action: ConflictSkip}, err
	}
}
