package tui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/Rayfts/WhyThis/internal/harness"
	"github.com/Rayfts/WhyThis/internal/risk"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

type Service interface {
	Line(context.Context, string) (evidence.Report, error)
	FileHistory(context.Context, string) (evidence.Report, error)
	Symbol(context.Context, string) (evidence.Report, error)
	Commit(context.Context, string) (evidence.Report, error)
	Similar(context.Context, string, int) (evidence.Report, error)
	Ask(context.Context, string) (evidence.Report, error)
	PR(context.Context, int) (evidence.Report, error)
	Risk(context.Context, string) (risk.Report, error)
	Harnesses(context.Context) []harness.Capabilities
}

type Options struct{ Plain bool }

func Run(ctx context.Context, svc Service, repoRoot string, in io.Reader, out io.Writer) error {
	return RunWithOptions(ctx, svc, repoRoot, in, out, Options{Plain: os.Getenv("NO_COLOR") != ""})
}

func RunWithOptions(ctx context.Context, svc Service, repoRoot string, in io.Reader, out io.Writer, opt Options) error {
	r := bufio.NewReader(in)
	for {
		if !opt.Plain {
			_, _ = fmt.Fprint(out, "\x1b[2J\x1b[H")
		}
		header(out, repoRoot)
		_, _ = fmt.Fprintln(out, "  [1] Line archaeology    [2] Symbol archaeology   [3] File history")
		_, _ = fmt.Fprintln(out, "  [4] Commit context      [5] Pull request          [6] Similar changes")
		_, _ = fmt.Fprintln(out, "  [7] Historical risk    [8] Ask                    [9] Harnesses")
		_, _ = fmt.Fprintln(out, "  [q] Quit")
		_, _ = fmt.Fprint(out, "\nwhythis> ")
		choice, err := readLine(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if choice == "q" || choice == "quit" || choice == "exit" {
			return nil
		}
		if err := execute(ctx, svc, repoRoot, choice, r, out); err != nil {
			_, _ = fmt.Fprintln(out, "\nERROR:", err)
		}
		_, _ = fmt.Fprint(out, "\nPress Enter to continue…")
		_, _ = readLine(r)
	}
}

func execute(ctx context.Context, svc Service, repoRoot, choice string, r *bufio.Reader, out io.Writer) error {
	switch choice {
	case "1", "line":
		v, err := prompt(r, out, "path:line[-line]")
		if err != nil {
			return err
		}
		rep, err := svc.Line(ctx, v)
		if err != nil {
			return err
		}
		renderReport(out, repoRoot, rep)
	case "2", "symbol":
		v, err := prompt(r, out, "symbol (Name, Type.Method, or path.go::Name)")
		if err != nil {
			return err
		}
		rep, err := svc.Symbol(ctx, v)
		if err != nil {
			return err
		}
		renderReport(out, repoRoot, rep)
	case "3", "history":
		v, err := prompt(r, out, "path")
		if err != nil {
			return err
		}
		rep, err := svc.FileHistory(ctx, v)
		if err != nil {
			return err
		}
		renderReport(out, repoRoot, rep)
	case "4", "commit":
		v, err := prompt(r, out, "commit SHA/ref")
		if err != nil {
			return err
		}
		rep, err := svc.Commit(ctx, v)
		if err != nil {
			return err
		}
		renderReport(out, repoRoot, rep)
	case "5", "pr":
		v, err := prompt(r, out, "PR number")
		if err != nil {
			return err
		}
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid PR number %q", v)
		}
		rep, err := svc.PR(ctx, n)
		if err != nil {
			return err
		}
		renderReport(out, repoRoot, rep)
	case "6", "similar":
		v, err := prompt(r, out, "commit SHA/ref")
		if err != nil {
			return err
		}
		rep, err := svc.Similar(ctx, v, 10)
		if err != nil {
			return err
		}
		renderReport(out, repoRoot, rep)
	case "7", "risk":
		v, err := prompt(r, out, "path")
		if err != nil {
			return err
		}
		rep, err := svc.Risk(ctx, v)
		if err != nil {
			return err
		}
		renderRisk(out, rep)
	case "8", "ask":
		v, err := prompt(r, out, "question")
		if err != nil {
			return err
		}
		rep, err := svc.Ask(ctx, v)
		if err != nil {
			return err
		}
		renderReport(out, repoRoot, rep)
	case "9", "harnesses":
		renderHarnesses(out, svc.Harnesses(ctx))
	default:
		return fmt.Errorf("unknown view %q", choice)
	}
	return nil
}
