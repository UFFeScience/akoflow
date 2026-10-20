import React, { useRef, type ImgHTMLAttributes } from "react";
import styles from "./ExpandableImage.module.css";

export default function ExpandableImage(
  props: ImgHTMLAttributes<HTMLImageElement>,
) {
  const dialog = useRef<HTMLDialogElement>(null);
  function openImage() {
    dialog.current?.showModal();
  }
  function closeImage() {
    dialog.current?.close();
  }
  return (
    <>
      <button
        type="button"
        className={styles.thumbnail}
        onClick={openImage}
        aria-label={`Enlarge image: ${props.alt ?? "documentation image"}`}
      >
        <img {...props} />
      </button>
      <dialog ref={dialog} className={styles.dialog} onClick={closeImage}>
        <button
          type="button"
          className={styles.close}
          onClick={closeImage}
          autoFocus
        >
          Close ×
        </button>
        <a
          href={props.src}
          target="_blank"
          rel="noreferrer"
          className={styles.original}
        >
          Open full resolution ↗
        </a>
        <img
          src={props.src}
          alt={props.alt ?? "Documentation image"}
          onClick={(event) => event.stopPropagation()}
        />
      </dialog>
    </>
  );
}
