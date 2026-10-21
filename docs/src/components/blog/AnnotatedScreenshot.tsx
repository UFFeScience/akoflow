import React, { useRef } from "react";
import ScreenshotCanvas, { type TutorialShot } from "./ScreenshotCanvas";
import styles from "./AnnotatedScreenshot.module.css";

export default function AnnotatedScreenshot({ shot }: { shot: TutorialShot }) {
  const dialog = useRef<HTMLDialogElement>(null);
  function openScreenshot() {
    dialog.current?.showModal();
  }
  function closeScreenshot() {
    dialog.current?.close();
  }
  return (
    <figure className={styles.figure}>
      <figcaption>
        <strong>{shot.title}</strong>
        <p>{shot.caption}</p>
      </figcaption>
      <button
        type="button"
        className={styles.thumbnail}
        onClick={openScreenshot}
        aria-label={`Enlarge annotated screenshot: ${shot.title}`}
      >
        <ScreenshotCanvas shot={shot} />
        <span className={styles.enlarge}>Enlarge screenshot and arrows ↗</span>
      </button>
      <ol className={styles.legend}>
        {shot.annotations.map((annotation) => (
          <li key={annotation.label}>{annotation.label}</li>
        ))}
      </ol>
      <dialog ref={dialog} className={styles.dialog} aria-label={shot.title}>
        <div className={styles.dialogToolbar}>
          <strong>{shot.title}</strong>
          <button type="button" onClick={closeScreenshot} autoFocus>
            Close ×
          </button>
        </div>
        <p className={styles.panHint}>
          On a small screen, scroll sideways to inspect each control.
        </p>
        <div className={styles.expanded}>
          <ScreenshotCanvas shot={shot} />
        </div>
        <ol className={styles.legend}>
          {shot.annotations.map((annotation) => (
            <li key={annotation.label}>{annotation.label}</li>
          ))}
        </ol>
      </dialog>
    </figure>
  );
}
