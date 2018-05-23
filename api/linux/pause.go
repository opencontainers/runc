package linux

import "context"

func (l *Libcontainer) Pause(ctx context.Context, id string) error {
	container, err := l.getContainer(id)
	if err != nil {
		return err
	}
	return container.Pause()
}

func (l *Libcontainer) Resume(ctx context.Context, id string) error {
	container, err := l.getContainer(id)
	if err != nil {
		return err
	}
	return container.Resume()
}
