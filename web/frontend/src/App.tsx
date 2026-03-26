import { useCallback } from "react";
import "./App.css";
import { useJob } from "./hooks/useJob";
import { UploadZone } from "./components/UploadZone";
import { Canvas } from "./components/Canvas";
import { Toolbar } from "./components/Toolbar";

function App() {
  const job = useJob();

  const handleFile = useCallback(
    (file: File) => {
      job.start(file);
    },
    [job.start]
  );

  const handleDownloadSVG = useCallback(() => {
    if (!job.info) return;
    const svgEl = document.querySelector(".canvas-svg");
    if (!svgEl) return;

    const svgData = new XMLSerializer().serializeToString(svgEl);
    const blob = new Blob([svgData], { type: "image/svg+xml" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "primitive.svg";
    a.click();
    URL.revokeObjectURL(url);
  }, [job.info]);

  const handleDownloadPNG = useCallback(() => {
    if (!job.info) return;
    const svgEl = document.querySelector(".canvas-svg");
    if (!svgEl) return;

    const svgData = new XMLSerializer().serializeToString(svgEl);
    const canvas = document.createElement("canvas");
    canvas.width = job.info.width;
    canvas.height = job.info.height;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const img = new Image();
    img.onload = () => {
      ctx.drawImage(img, 0, 0);
      canvas.toBlob((blob) => {
        if (!blob) return;
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = "primitive.png";
        a.click();
        URL.revokeObjectURL(url);
      }, "image/png");
    };
    img.src = "data:image/svg+xml;base64," + btoa(svgData);
  }, [job.info]);

  return (
    <div className="app">
      <header className="header">
        <h1>PRIMITIVE</h1>
      </header>
      <main className="main">
        {job.state === "idle" && <UploadZone onFile={handleFile} />}
        {job.state !== "idle" && job.info && (
          <div className="canvas-wrapper">
            <Canvas
              info={job.info}
              shapes={job.shapes}
              shapeCount={job.shapeCount}
              totalCount={job.totalCount}
            />
            <Toolbar
              state={job.state}
              onStop={job.stop}
              onReset={job.reset}
              onDownloadSVG={handleDownloadSVG}
              onDownloadPNG={handleDownloadPNG}
            />
          </div>
        )}
      </main>
    </div>
  );
}

export default App;
