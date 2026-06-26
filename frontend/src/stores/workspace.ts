import { defineStore } from 'pinia'

export const useWorkspaceStore = defineStore('workspace', {
  state: () => ({
    selectedBlockId: null as string | null,
  }),
  actions: {
    selectBlock(blockId: string | null) {
      this.selectedBlockId = blockId
    },
  },
})
