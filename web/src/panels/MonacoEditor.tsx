import { Editor, type OnChange } from "@monaco-editor/react";
import { useCallback } from "react";

export interface MonacoEditorProps {
  value: string;
  language: string;
  path?: string;
  onChange: (value: string) => void;
}

export function MonacoEditor({
  value,
  language,
  path,
  onChange,
}: MonacoEditorProps) {
  const handleChange = useCallback<OnChange>(
    (next) => {
      onChange(next ?? "");
    },
    [onChange],
  );

  return (
    <Editor
      value={value}
      language={language}
      path={path!}
      theme="zanbato"
      onChange={handleChange}
      options={{
        minimap: { enabled: false },
        fontSize: 13,
        tabSize: 2,
        insertSpaces: true,
        scrollBeyondLastLine: false,
        automaticLayout: true,
        renderLineHighlight: "all",
        wordWrap: "off",
      }}
    />
  );
}
