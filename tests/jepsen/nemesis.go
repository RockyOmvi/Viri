package jepsen

import (
	"context"
	"fmt"
	"math/rand"
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
			"viri-validator-0", "viri-validator-1", "viri-validator-2", "viri-validator-3",
		},
	}
}

func GetNetworkName() string {
	return "testnet_default"
}

func init() {
	rand.Seed(time.Now().UnixNano())
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
	network string
}

func (p *PartitionNemesis) Inject(ctx context.Context) string {
	idx1 := rand.Intn(len(p.containers))
	idx2 := rand.Intn(len(p.containers))
	for idx2 == idx1 {
		idx2 = rand.Intn(len(p.containers))
	}
	c1 := p.containers[idx1]

	p.runCmd(ctx, "network", "connect", p.network, c1)
	time.Sleep(500 * time.Millisecond)

	err := p.runCmd(ctx, "network", "disconnect", p.network, c1)
	if err != nil {
		return fmt.Sprintf("partition: failed to disconnect %s: %v", c1, err)
	}
	time.Sleep(5 * time.Second)
	err = p.runCmd(ctx, "network", "connect", p.network, c1)
	if err != nil {
		return fmt.Sprintf("partition: failed to reconnect %s: %v", c1, err)
	}
	return fmt.Sprintf("partition: isolated %s from peers for 5s", c1)
}

type KillNemesis struct {
	*DockerNemesis
}

func (k *KillNemesis) Inject(ctx context.Context) string {
	target := k.containers[rand.Intn(len(k.containers))]
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
	i1 := rand.Intn(len(p.containers))
	i2 := rand.Intn(len(p.containers))
	for i2 == i1 {
		i2 = rand.Intn(len(p.containers))
	}
	targets := []string{p.containers[i1], p.containers[i2]}
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

func NewRandomNemesis(d *DockerNemesis, network string) *RandomNemesis {
	return &RandomNemesis{
		nemeses: []Nemesis{
			&PartitionNemesis{DockerNemesis: d, network: network},
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
	n := r.nemeses[rand.Intn(len(r.nemeses))]
	return n.Inject(ctx)
}
