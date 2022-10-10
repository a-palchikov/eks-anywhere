package executables

import "context"

type DockerContainer interface {
	Init(ctx context.Context) error
	Close(ctx context.Context) error
	ContainerName() string
}

func NewDockerExecutableBuilder(dockerContainer DockerContainer) *dockerExecutableBuilder {
	return &dockerExecutableBuilder{
		container: dockerContainer,
	}
}

type dockerExecutableBuilder struct {
	container DockerContainer
}

func (d *dockerExecutableBuilder) Build(cmd ...string) Executable {
	return NewDockerExecutable(cmd, d.container.ContainerName())
}

func (b *dockerExecutableBuilder) Init(ctx context.Context) (Closer, error) {
	return b.container.Close, b.container.Init(ctx)
}
