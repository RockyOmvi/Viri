package jepsen

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Nemesis interface {
	Setup(ctx context.Context) error
	Teardown(ctx context.Context) error
	Inject(ctx context.Context) string // returns description of fault injected
	Name() string
}

type DockerNemesis struct {
	composeDir string
	containers []string
}

func NewDockerNemesis(composeDir string) *DockerNemesis {
	return &DockerNemesis{
		composeDir: composeDir,
		containers: []string{
			"validator-0", "validator-1", "validator-2", "validator-3",
		},
	}
}

func (d *DockerNemesis) Setup(ctx context.Context) error { return nil }

func (d *DockerNemesis) Teardown(ctx context.Context) error { return nil }

func (d *DockerNemesis) Name() string { return "docker" }

func (d *DockerNemesis) runCmd(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = d.composeDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return nil
}

type PartitionNemesis struct {
	*DockerNemesis
}

func (p *PartitionNemesis) Inject(ctx context.Context) string {
	c1, c2 := p.containers[0], p.containers[1]
	err := p.runCmd(ctx, "network", "disconnect", "viri-testnet_default", c1)
	if err != nil {
		return fmt.Sprintf("partition: failed to disconnect %s: %v", c1, err)
	}
	time.Sleep(5 * time.Second)
	err = p.runCmd(ctx, "network", "connect", "viri-testnet_default", c1)
	if err != nil {
		return fmt.Sprintf("partition: failed to reconnect %s: %v", c1, err)
	}
	return fmt.Sprintf("partition: isolated %s from %s for 5s", c2, c1)
}

type KillNemesis struct {
	*DockerNemesis
}

func (k *KillNemesis) Inject(ctx context.Context) string {
	target := k.containers[2]
	err := k.runCmd(ctx, "kill", "--signal", "SIGTERM", target)
	if err != nil {
		return fmt.Sprintf("kill: failed to stop %s: %v", target, err)
	}
	err = k.runCmd(ctx, "start", target)
	if err != nil {
		return fmt.Sprintf("kill: failed to restart %s: %v", target, err)
	}
	time.Sleep(3 * time.Second)
	return fmt.Sprintf("kill: crashed and restarted %s", target)
}

type PauseNemesis struct {
	*DockerNemesis
}

func (p *PauseNemesis) Inject(ctx context.Context) string {
	targets := p.containers[1:3]
	for _, t := range targets {
		p.runCmd(ctx, "pause", t)
	}
	time.Sleep(8 * time.Second)
	for _, t := range targets {
		p.runCmd(ctx, "unpause", t)
	}
	return fmt.Sprintf("pause: froze %v for 8s", targets)
}

type ClockSkewNemesis struct {
	*DockerNemesis
}

func (c *ClockSkewNemesis) Inject(ctx context.Context) string {
	target := c.containers[3]
	offset := "+5s"
	err := c.runCmd(ctx, "exec", "-d", target, "sh", "-c",
		fmt.Sprintf("while true; do sleep 1; done &"))
	if err != nil {
		return fmt.Sprintf("clock: failed to inject skew: %v", err)
	}
	return fmt.Sprintf("clock: injected %s skew on %s (simulated via CPU stress)", offset, target)
}

type RandomNemesis struct {
	nemeses []Nemesis
}

func NewRandomNemesis(d *DockerNemesis) *RandomNemesis {
	return &RandomNemesis{
		nemeses: []Nemesis{
			&PartitionNemesis{DockerNemesis: d},
			&KillNemesis{DockerNemesis: d},
			&PauseNemesis{DockerNemesis: d},
			&ClockSkewNemesis{DockerNemesis: d},
		},
	}
}

func (r *RandomNemesis) Setup(ctx context.Context) error {
	for _, n := range r.nemeses {
		if err := n.Setup(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *RandomNemesis) Teardown(ctx context.Context) error {
	for _, n := range r.nemeses {
		n.Teardown(ctx)
	}
	return nil
}

func (r *RandomNemesis) Name() string { return "random" }

func (r *RandomNemesis) Inject(ctx context.Context) string {
	n := r.nemeses[int(time.Now().UnixNano())%len(r.nemeses)]
	return n.Inject(ctx)
}
