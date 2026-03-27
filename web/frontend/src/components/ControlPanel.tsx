import { useState, useEffect } from "react";
import type { JobParams } from "../hooks/useJob";

interface ControlPanelProps {
  file: File;
  onStart: (params: JobParams, delay: number) => void;
  onBack: () => void;
}

const SHAPE_TYPES = [
  { value: 0, label: "Combo" },
  { value: 1, label: "Triangle" },
  { value: 2, label: "Rectangle" },
  { value: 3, label: "Ellipse" },
  { value: 4, label: "Circle" },
  { value: 5, label: "Rotated Rect" },
  { value: 6, label: "Bezier" },
  { value: 7, label: "Rotated Ellipse" },
  { value: 8, label: "Polygon" },
];

export function ControlPanel({ file, onStart, onBack }: ControlPanelProps) {
  const [mode, setMode] = useState(1);
  const [count, setCount] = useState(100);
  const [alpha, setAlpha] = useState(128);
  const [autoAlpha, setAutoAlpha] = useState(false);
  const [inputSize, setInputSize] = useState(256);
  const [outputSize, setOutputSize] = useState(1024);
  const [delay, setDelay] = useState(0);

  const [thumbnailUrl, setThumbnailUrl] = useState("");
  useEffect(() => {
    const url = URL.createObjectURL(file);
    setThumbnailUrl(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  const handleStart = () => {
    onStart(
      { mode, count, alpha: autoAlpha ? 0 : alpha, inputSize, outputSize },
      delay
    );
  };

  return (
    <div className="control-panel">
      <div className="control-panel__preview">
        <img
          src={thumbnailUrl}
          alt="Preview"
          className="control-panel__thumbnail"
        />
      </div>
      <div className="control-panel__form">
        <div className="control-group">
          <label className="control-label">Shape Type</label>
          <select
            className="control-select"
            value={mode}
            onChange={(e) => setMode(Number(e.target.value))}
          >
            {SHAPE_TYPES.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>
        </div>

        <div className="control-group">
          <label className="control-label">Shape Count</label>
          <input
            type="number"
            className="control-input"
            value={count}
            min={1}
            max={1000}
            onChange={(e) =>
              setCount(Math.max(1, Math.min(1000, Number(e.target.value))))
            }
          />
        </div>

        <div className="control-group">
          <label className="control-label">Alpha</label>
          <div className="control-row">
            <input
              type="range"
              className="control-slider"
              value={alpha}
              min={1}
              max={255}
              disabled={autoAlpha}
              onChange={(e) => setAlpha(Number(e.target.value))}
            />
            <input
              type="number"
              className="control-input control-input--small"
              value={alpha}
              min={1}
              max={255}
              disabled={autoAlpha}
              onChange={(e) =>
                setAlpha(Math.max(1, Math.min(255, Number(e.target.value))))
              }
            />
            <label className="control-checkbox">
              <input
                type="checkbox"
                checked={autoAlpha}
                onChange={(e) => setAutoAlpha(e.target.checked)}
              />
              Auto
            </label>
          </div>
        </div>

        <div className="control-group">
          <label className="control-label">Input Size</label>
          <input
            type="number"
            className="control-input"
            value={inputSize}
            min={64}
            max={1024}
            onChange={(e) =>
              setInputSize(Math.max(64, Math.min(1024, Number(e.target.value))))
            }
          />
          <span className="control-hint">
            Processing resolution — smaller = faster
          </span>
        </div>

        <div className="control-group">
          <label className="control-label">Output Size</label>
          <input
            type="number"
            className="control-input"
            value={outputSize}
            min={256}
            max={4096}
            onChange={(e) =>
              setOutputSize(
                Math.max(256, Math.min(4096, Number(e.target.value)))
              )
            }
          />
          <span className="control-hint">Render resolution</span>
        </div>

        <div className="control-group">
          <label className="control-label">
            Playback Speed
            <span className="control-value">
              {delay === 0 ? "Instant" : delay >= 500 ? "Slow" : `${delay}ms`}
            </span>
          </label>
          <input
            type="range"
            className="control-slider"
            value={delay}
            min={0}
            max={500}
            onChange={(e) => setDelay(Number(e.target.value))}
          />
          <div className="control-range-labels">
            <span>Instant</span>
            <span>Slow</span>
          </div>
        </div>

        <div className="control-panel__buttons">
          <button className="btn btn--secondary" onClick={onBack}>
            Back
          </button>
          <button className="btn btn--primary" onClick={handleStart}>
            Start
          </button>
        </div>
      </div>
    </div>
  );
}
