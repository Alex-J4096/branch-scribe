<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { EditorContent, useEditor } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import { Bold, Heading2, Italic, List, ListOrdered, Redo2, Undo2 } from 'lucide-vue-next'
import type { Editor } from '@tiptap/core'

const props = defineProps<{
  modelValue: string
  contentFormat?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  selectionChange: [value: string]
}>()

const editorContent = computed(() => normalizeEditorContent(props.modelValue, props.contentFormat))
const lastTextSelection = ref<{ from: number; to: number; text: string } | null>(null)

const editor = useEditor({
  extensions: [StarterKit],
  content: editorContent.value,
  editorProps: {
    attributes: {
      class: 'rich-editor__content',
    },
  },
  onUpdate: ({ editor }) => {
    emit('update:modelValue', editor.getHTML())
    captureSelection(editor)
  },
  onSelectionUpdate: ({ editor }) => {
    captureSelection(editor)
  },
})

watch(
  editorContent,
  (value) => {
    const instance = editor.value
    if (!instance || instance.getHTML() === value) return
    instance.commands.setContent(value, { emitUpdate: false })
  },
)

onBeforeUnmount(() => {
  editor.value?.destroy()
})

function normalizeEditorContent(content: string, format?: string) {
  if (!content.trim()) return '<p></p>'
  if (format === 'html' || isLikelyHTML(content)) return content

  return content
    .split(/\n{2,}/)
    .map((paragraph) => `<p>${escapeHTML(paragraph).replace(/\n/g, '<br>')}</p>`)
    .join('')
}

function isLikelyHTML(value: string) {
  return /<\/?[a-z][\s\S]*>/i.test(value)
}

function escapeHTML(value: string) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;')
}

function captureSelection(instance?: Editor | null) {
  instance ??= editor.value
  if (!instance) return

  const { from, to } = instance.state.selection
  const text = instance.state.doc.textBetween(from, to, '\n').trim()
  if (from !== to && text) {
    lastTextSelection.value = { from, to, text }
    emit('selectionChange', text)
    return
  }
  emit('selectionChange', '')
}

function getSelectedText() {
  return lastTextSelection.value?.text ?? ''
}

function replaceSelectionWithHTML(html: string) {
  const instance = editor.value
  const selection = lastTextSelection.value
  if (!instance || !selection) return null

  const maxPosition = instance.state.doc.content.size
  if (selection.from < 0 || selection.to > maxPosition || selection.from >= selection.to) {
    return null
  }

  instance.chain().focus().setTextSelection({ from: selection.from, to: selection.to }).insertContent(html).run()
  const nextHTML = instance.getHTML()
  emit('update:modelValue', nextHTML)
  lastTextSelection.value = null
  emit('selectionChange', '')
  return nextHTML
}

defineExpose({
  getSelectedText,
  replaceSelectionWithHTML,
})
</script>

<template>
  <div class="rich-editor">
    <div class="rich-editor__toolbar" role="toolbar">
      <button
        class="icon-button"
        :class="{ 'is-active': editor?.isActive('bold') }"
        type="button"
        title="粗体"
        @click="editor?.chain().focus().toggleBold().run()"
      >
        <Bold :size="15" aria-hidden="true" />
      </button>
      <button
        class="icon-button"
        :class="{ 'is-active': editor?.isActive('italic') }"
        type="button"
        title="斜体"
        @click="editor?.chain().focus().toggleItalic().run()"
      >
        <Italic :size="15" aria-hidden="true" />
      </button>
      <button
        class="icon-button"
        :class="{ 'is-active': editor?.isActive('heading', { level: 2 }) }"
        type="button"
        title="二级标题"
        @click="editor?.chain().focus().toggleHeading({ level: 2 }).run()"
      >
        <Heading2 :size="15" aria-hidden="true" />
      </button>
      <button
        class="icon-button"
        :class="{ 'is-active': editor?.isActive('bulletList') }"
        type="button"
        title="无序列表"
        @click="editor?.chain().focus().toggleBulletList().run()"
      >
        <List :size="15" aria-hidden="true" />
      </button>
      <button
        class="icon-button"
        :class="{ 'is-active': editor?.isActive('orderedList') }"
        type="button"
        title="有序列表"
        @click="editor?.chain().focus().toggleOrderedList().run()"
      >
        <ListOrdered :size="15" aria-hidden="true" />
      </button>
      <span class="rich-editor__spacer"></span>
      <button class="icon-button" type="button" title="撤销" @click="editor?.chain().focus().undo().run()">
        <Undo2 :size="15" aria-hidden="true" />
      </button>
      <button class="icon-button" type="button" title="重做" @click="editor?.chain().focus().redo().run()">
        <Redo2 :size="15" aria-hidden="true" />
      </button>
    </div>
    <EditorContent :editor="editor" />
  </div>
</template>
