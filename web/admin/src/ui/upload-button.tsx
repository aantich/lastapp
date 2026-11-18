import { useRef } from "react";
import { Button, type ButtonProps } from "./button";

// A button that opens the browser's file chooser (a build folder, a CSV):
// the native file input stays hidden and the button is what people and
// assistive tech meet. `directory` picks a whole folder (each File keeps its
// webkitRelativePath). The input is cleared once `onFiles` settles, so the
// same folder can be chosen again.
export function UploadButton({
  onFiles,
  directory,
  multiple = directory,
  accept,
  ...button
}: Omit<ButtonProps, "onClick"> & {
  onFiles: (files: FileList) => void | Promise<void>;
  directory?: boolean;
  multiple?: boolean;
  accept?: string;
}) {
  const input = useRef<HTMLInputElement>(null);
  return (
    <>
      <Button
        {...button}
        onClick={() => {
          if (directory) input.current?.setAttribute("webkitdirectory", "");
          input.current?.click();
        }}
      />
      <input
        ref={input}
        type="file"
        multiple={multiple}
        accept={accept}
        hidden
        onChange={async (e) => {
          const target = e.currentTarget,
            files = target.files;
          try {
            if (files?.length) await onFiles(files);
          } finally {
            target.value = "";
          }
        }}
      />
    </>
  );
}
