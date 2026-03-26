import { useMemo } from "react";
import type { JobInfo } from "../hooks/useJob";

interface CanvasProps {
  info: JobInfo;
  shapes: string[];
  shapeCount: number;
  totalCount: number;
}

export function Canvas({ info, shapes, shapeCount, totalCount }: CanvasProps) {
  const svgContent = useMemo(() => {
    const bg = `<rect width="${info.width}" height="${info.height}" fill="${info.background}" />`;
    const group = shapes.join("\n");
    return `${bg}\n${group}`;
  }, [info, shapes]);

  return (
    <div className="canvas-container">
      <svg
        viewBox={`0 0 ${info.width} ${info.height}`}
        className="canvas-svg"
        dangerouslySetInnerHTML={{ __html: svgContent }}
      />
      <div className="canvas-counter">
        {shapeCount} / {totalCount} shapes
      </div>
    </div>
  );
}
