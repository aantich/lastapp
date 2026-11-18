import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Upload } from "lucide-react";
import { UploadButton } from "../upload-button";
import { LoadingScreen } from "../loading-screen";
import { Inline, Stack } from "../layout";

// upload-button.tsx: a Button that opens the file chooser (a build folder
// for Dispatch Board's next release) over a hidden native input.
// loading-screen.tsx: the whole-window wait before the console draws.
const meta = { title: "Primitives/Upload and loading" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

function Folder() {
  const [chosen, setChosen] = useState("No folder chosen yet.");
  return (
    <Stack gap="sm">
      <Inline>
        <UploadButton
          directory
          variant="primary"
          icon={<Upload size={16} aria-hidden />}
          onFiles={(files) =>
            setChosen(
              `${files.length} files from ${files[0].webkitRelativePath.split("/")[0]}`,
            )
          }
        >
          Upload build folder
        </UploadButton>
      </Inline>
      <p>{chosen}</p>
    </Stack>
  );
}

export const BuildFolder: Story = { render: () => <Folder /> };

export const Loading: Story = {
  parameters: { layout: "fullscreen" },
  render: () => <LoadingScreen label="Opening Lastapp…" />,
};
