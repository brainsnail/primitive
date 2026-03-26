# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Primitive reproduces images using geometric primitives. It iteratively finds optimal shapes (triangles, rectangles, ellipses, etc.) via hill climbing to minimize RMSE between a target image and the current rendered output.

## Build & Run

```bash
go build -o primitive .
go vet ./...

# Basic usage
./primitive -i input.png -o output.png -n 100

# Stdin/stdout support (SVG output)
cat input.png | ./primitive -i - -o - -n 100 > output.svg
```

There are no tests in this repository.

## CLI Flags

Required: `-i <input>`, `-o <output>`, `-n <count>`

Key options: `-m <mode>` (0=combo, 1=triangle, 2=rect, 3=ellipse, 4=circle, 5=rotatedrect, 6=beziers, 7=rotatedellipse, 8=polygon), `-a <alpha>` (0=auto), `-r <input-resize>` (default 256), `-s <output-size>` (default 1024), `-j <workers>` (default all CPUs), `-bg <hex>`.

Output formats determined by extension: `.png`, `.jpg`, `.svg`, `.gif` (requires ImageMagick `convert`). Use `%d` in output path for per-frame files.

## Architecture

**Entry point**: `main.go` - CLI argument parsing, image loading, orchestration loop.

**Core package** (`primitive/`):

- **Model** (`model.go`) - Orchestrates the algorithm. Holds target/current images, accumulated shapes, scores, and worker pool. `Step()` runs one iteration: dispatches workers in parallel, picks the best shape, adds it via `Add()`. Also generates SVG and animation frames.

- **Worker** (`worker.go`) - Each worker independently searches for shapes. `BestHillClimbState()` generates M random shapes, hill-climbs each, returns the best. Workers run as goroutines and communicate results via channels.

- **Shape interface** (`shape.go`) - All shapes implement `Rasterize() []Scanline`, `Copy()`, `Mutate()`, `Draw()`, `SVG()`. Implementations: Triangle, Rectangle, Ellipse, Circle, RotatedRectangle, RotatedEllipse, QuadraticBezier, Polygon.

- **State** (`state.go`) - Wraps a shape for optimization. Implements the `Annealable` interface with `Energy()` (computes RMSE via `differencePartial`). Bridges the optimizer and the shape.

- **Optimization** (`optimize.go`) - `HillClimb()` mutates shapes and keeps improvements. `Anneal()` does simulated annealing with temperature-based acceptance. The algorithm uses hill climbing with multiple random starting points.

- **Rasterization** (`scanline.go`, `raster.go`, `core.go`) - Scanline-based pure Go rasterizer. `computeColor()` calculates the optimal color for a shape. `differencePartial()` efficiently scores by only recomputing changed pixels.

**Algorithm flow per step**: Generate random shapes -> hill climb each -> pick best across all workers -> compute optimal color -> composite onto current image -> record score.

## Module

`github.com/fogleman/primitive` - Go 1.24. Key dependency: `github.com/fogleman/gg` for high-quality drawing context used in final output rendering (separate from the scanline-based scoring rasterizer).
