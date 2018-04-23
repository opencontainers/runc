package linux

func (l *Libcontainer) Pause(id string) error {
	container, err := l.getContainer(id)
	if err != nil {
		return err
	}
	return container.Pause()
}

func (l *Libcontainer) Resume(id string) error {
	container, err := l.getContainer(id)
	if err != nil {
		return err
	}
	return container.Resume()
}
