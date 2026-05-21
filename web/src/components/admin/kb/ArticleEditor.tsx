import { useCallback, useEffect, useRef, useState } from "react";
import { useEditor, EditorContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Image from "@tiptap/extension-image";
import Link from "@tiptap/extension-link";
import { Bold, Code, Image as ImageIcon, Italic, Link2, List, ListOrdered, Quote } from "lucide-react";

import { Button } from "@/components/ui/button";
import { uploadKBImage } from "@/api/kbAdmin";

type Props = {
  initialHTML: string;
  articleId?: number;
  onChange: (html: string) => void;
};

export function ArticleEditor({ initialHTML, articleId, onChange }: Props) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const editor = useEditor({
    extensions: [
      StarterKit.configure({ heading: { levels: [1, 2, 3] } }),
      Image,
      Link.configure({ openOnClick: false, autolink: true }),
    ],
    content: initialHTML,
    onUpdate: ({ editor }) => onChange(editor.getHTML()),
  });
  const [uploading, setUploading] = useState(false);

  useEffect(() => {
    if (editor && initialHTML && editor.getHTML() !== initialHTML) {
      editor.commands.setContent(initialHTML);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editor]);

  const pickImage = useCallback(() => fileInputRef.current?.click(), []);

  const onImageChange = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file || !editor) return;
    setUploading(true);
    try {
      const { url } = await uploadKBImage(file, articleId);
      editor.chain().focus().setImage({ src: url, alt: file.name }).run();
    } catch (err) {
      console.error(err);
    } finally {
      setUploading(false);
    }
  }, [editor, articleId]);

  if (!editor) return null;

  return (
    <div className="space-y-2 rounded-md border border-border">
      <div className="flex flex-wrap items-center gap-1 border-b border-border bg-card px-2 py-1.5">
        <ToolbarBtn active={editor.isActive("bold")} onClick={() => editor.chain().focus().toggleBold().run()} aria-label="Bold">
          <Bold className="h-4 w-4" />
        </ToolbarBtn>
        <ToolbarBtn active={editor.isActive("italic")} onClick={() => editor.chain().focus().toggleItalic().run()} aria-label="Italic">
          <Italic className="h-4 w-4" />
        </ToolbarBtn>
        <ToolbarBtn active={editor.isActive("code")} onClick={() => editor.chain().focus().toggleCode().run()} aria-label="Inline code">
          <Code className="h-4 w-4" />
        </ToolbarBtn>
        <Separator />
        <ToolbarBtn active={editor.isActive("heading", { level: 1 })} onClick={() => editor.chain().focus().toggleHeading({ level: 1 }).run()} aria-label="Heading 1">H1</ToolbarBtn>
        <ToolbarBtn active={editor.isActive("heading", { level: 2 })} onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()} aria-label="Heading 2">H2</ToolbarBtn>
        <ToolbarBtn active={editor.isActive("heading", { level: 3 })} onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()} aria-label="Heading 3">H3</ToolbarBtn>
        <Separator />
        <ToolbarBtn active={editor.isActive("bulletList")} onClick={() => editor.chain().focus().toggleBulletList().run()} aria-label="Bullet list">
          <List className="h-4 w-4" />
        </ToolbarBtn>
        <ToolbarBtn active={editor.isActive("orderedList")} onClick={() => editor.chain().focus().toggleOrderedList().run()} aria-label="Numbered list">
          <ListOrdered className="h-4 w-4" />
        </ToolbarBtn>
        <ToolbarBtn active={editor.isActive("blockquote")} onClick={() => editor.chain().focus().toggleBlockquote().run()} aria-label="Quote">
          <Quote className="h-4 w-4" />
        </ToolbarBtn>
        <Separator />
        <ToolbarBtn
          active={editor.isActive("link")}
          onClick={() => {
            const url = window.prompt("Link URL", editor.getAttributes("link").href ?? "");
            if (url === null) return;
            if (url === "") editor.chain().focus().unsetLink().run();
            else editor.chain().focus().setLink({ href: url }).run();
          }}
          aria-label="Link"
        >
          <Link2 className="h-4 w-4" />
        </ToolbarBtn>
        <ToolbarBtn onClick={pickImage} aria-label="Image">
          <ImageIcon className="h-4 w-4" />
        </ToolbarBtn>
        {uploading && <span className="ml-2 text-xs text-muted-foreground">Uploading…</span>}
      </div>
      <EditorContent editor={editor} className="prose prose-invert max-w-none px-4 py-3 min-h-[300px]" />
      <input
        ref={fileInputRef}
        type="file"
        accept="image/png,image/jpeg,image/gif,image/webp,image/svg+xml"
        className="hidden"
        onChange={onImageChange}
      />
    </div>
  );
}

function Separator() {
  return <span className="mx-1 h-5 w-px bg-border" aria-hidden />;
}

function ToolbarBtn({
  active, children, ...rest
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { active?: boolean }) {
  return (
    <Button type="button" variant={active ? "default" : "ghost"} size="sm" className="h-7 px-2" {...rest}>
      {children}
    </Button>
  );
}
