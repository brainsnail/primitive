import type { JobState } from "../hooks/useJob";

interface ToolbarProps {
  state: JobState;
  onStop: () => void;
  onReset: () => void;
  onDownloadSVG: () => void;
  onDownloadPNG: () => void;
}

export function Toolbar({ state, onStop, onReset, onDownloadSVG, onDownloadPNG }: ToolbarProps) {
  if (state === "processing") {
    return (
      <div className="toolbar">
        <span className="toolbar__status">Processing...</span>
        <button className="btn btn--danger" onClick={onStop}>
          Stop
        </button>
      </div>
    );
  }

  if (state === "done") {
    return (
      <div className="toolbar">
        <span className="toolbar__status toolbar__status--done">Done</span>
        <div className="toolbar__actions">
          <button className="btn btn--primary" onClick={onDownloadSVG}>
            Download SVG
          </button>
          <button className="btn btn--primary" onClick={onDownloadPNG}>
            Download PNG
          </button>
          <button className="btn btn--secondary" onClick={onReset}>
            New
          </button>
        </div>
      </div>
    );
  }

  return null;
}
