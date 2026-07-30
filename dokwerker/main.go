package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pydpll/errorutils"
	"github.com/urfave/cli/v3"
)

func getBoilerplateDir() (string, error) {
	skriptujo := os.Getenv("scriptLoc")
	if skriptujo == "" {
		return "", fmt.Errorf("environment variable is not set for scriptLoc")
	}
	return filepath.Join(skriptujo, "boilerplate", "docker"), nil
}

// runCmd executes a shell command with current UID/GID environment variables injected
func runCmd(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Inject current user's UID and GID so Docker volumes never get permission-locked
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("UID=%d", os.Getuid()),
		fmt.Sprintf("GID=%d", os.Getgid()),
	)

	return cmd.Run()
}

// copyFile helper for initializing boilerplate files
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// getServiceMapping returns the default compose service name based on stack type
func getServiceMapping(stack string) (string, error) {
	switch stack {
	case "go":
		return "go-dev", nil
	case "ts":
		return "obsidian-dev", nil
	default:
		return "", fmt.Errorf("unknown stack type '%s' (use 'go' or 'ts')", stack)
	}
}

// InitAction copies boilerplate docker-compose and dockerfile to current dir
func InitAction(ctx context.Context, c *cli.Command) error {
	boilerplateDir, err := getBoilerplateDir()
	if err != nil {
		return err
	}
	stack := c.String("stack")
	fmt.Printf("Initializing project with %s stack boilerplate...\n", stack)

	var srcCompose, srcDockerfile string
	switch stack {
	case "go":
		srcCompose = filepath.Join(boilerplateDir, "go-docker-compose.yml")
		srcDockerfile = filepath.Join(boilerplateDir, "go-dockfile")
	case "ts":
		srcCompose = filepath.Join(boilerplateDir, "ts-docker-compose.yml")
		srcDockerfile = filepath.Join(boilerplateDir, "ts-dockerfile")
	default:
		return fmt.Errorf("unsupported stack: %s", stack)
	}

	if err := copyFile(srcCompose, "docker-compose.yml"); err != nil {
		return fmt.Errorf("failed to copy compose file (is $skriptujo set correctly?): %w", err)
	}
	if err := copyFile(srcDockerfile, "Dockerfile"); err != nil {
		return fmt.Errorf("failed to copy dockerfile: %w", err)
	}

	fmt.Println("Boilerplate files successfully created (docker-compose.yml & Dockerfile).")
	return nil
}

// UpAction builds and starts the compose stack
func UpAction(ctx context.Context, c *cli.Command) error {
	project := c.String("project")
	args := []string{"compose"}
	if project != "" {
		args = append(args, "-p", project)
	}
	args = append(args, "up", "-d", "--build")

	fmt.Printf("Building and starting docker compose stack...\n")
	return runCmd(ctx, "docker", args...)
}

// ShellAction jumps straight into the container's bash shell
func ShellAction(ctx context.Context, c *cli.Command) error {
	stack := c.String("stack")
	project := c.String("project")
	service, err := getServiceMapping(stack)
	if err != nil {
		return err
	}

	args := []string{"compose"}
	if project != "" {
		args = append(args, "-p", project)
	}
	args = append(args, "exec", service, "bash")

	fmt.Printf("Entering container shell for service '%s'...\n", service)
	err = runCmd(ctx, "docker", args...)
	if err != nil {
		return fmt.Errorf("shell exec failed: %w (forgot to run 'dockerhelper init --stack %s' first, or use 'fresh' to do it automatically?)", err, stack)
	}
	return nil
}

// DownAction tears down the compose stack
func DownAction(ctx context.Context, c *cli.Command) error {
	project := c.String("project")
	args := []string{"compose"}
	if project != "" {
		args = append(args, "-p", project)
	}
	args = append(args, "down")

	fmt.Println("Stopping and cleaning up stack containers...")
	return runCmd(ctx, "docker", args...)
}

func main() {
	cmd := &cli.Command{
		Name:  "dokwerker",
		Usage: "Generalized workflow manager for Go and TypeScript Docker development",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "stack",
				Aliases: []string{"s"},
				Value:   "go",
				Usage:   "Target technology stack (`go` or `ts`)",
			},
			&cli.StringFlag{
				Name:    "project",
				Aliases: []string{"p"},
				Value:   "",
				Usage:   "Optional Docker Compose project name (e.g., `bibtex-1`)",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "init",
				Usage:  "Copy boilerplate Docker files into the current directory",
				Action: InitAction,
			},
			{
				Name:   "up",
				Usage:  "Build and launch the Docker Compose stack in the background",
				Action: UpAction,
			},
			{
				Name:   "shell",
				Usage:  "Exec into the running development container's bash prompt",
				Action: ShellAction,
			},
			{
				Name:   "down",
				Usage:  "Stop and remove project containers",
				Action: DownAction,
			},
			{
				Name:    "fresh",
				Aliases: []string{"reset"},
				Usage:   "Initialize (if needed), bring up the build, and jump right into the shell",
				Action: func(ctx context.Context, c *cli.Command) error {
					// Optional: auto-init if files don't exist locally
					if _, err := os.Stat("docker-compose.yml"); err != nil {
						if !os.IsNotExist(err) {
							return fmt.Errorf("could not check for docker-compose.yml: %w", err)
						}
						if err := InitAction(ctx, c); err != nil {
							return fmt.Errorf("automatic initialization failed: %w", err)
						}
					}
					if err := UpAction(ctx, c); err != nil {
						return err
					}
					return ShellAction(ctx, c)
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		errorutils.ExitOnFail(err)
	}
}
