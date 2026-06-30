import { createRouter, createWebHistory } from 'vue-router'

import ProjectList from '@/views/ProjectList.vue'
import ProjectWorkspace from '@/views/ProjectWorkspace.vue'
import ModelProfileSettings from '@/views/ModelProfileSettings.vue'
import CanonManager from '@/views/CanonManager.vue'
import MemoryManager from '@/views/MemoryManager.vue'
import BlockTool from '@/views/BlockTool.vue'
import ForeshadowingManager from '@/views/ForeshadowingManager.vue'
import TimelineManager from '@/views/TimelineManager.vue'
import TransferManager from '@/views/TransferManager.vue'
import PromptSettings from '@/views/PromptSettings.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'projects',
      component: ProjectList,
    },
    {
      path: '/projects/:projectId',
      name: 'workspace',
      component: ProjectWorkspace,
    },
    {
      path: '/projects/:projectId/blocks/:blockId/tool',
      name: 'block-tool',
      component: BlockTool,
    },
    {
      path: '/settings/model-profiles',
      name: 'model-profiles',
      component: ModelProfileSettings,
    },
    {
      path: '/projects/:projectId/canon/:entityType',
      name: 'canon-manager',
      component: CanonManager,
    },
    {
      path: '/projects/:projectId/memory',
      name: 'memory-manager',
      component: MemoryManager,
    },
    {
      path: '/projects/:projectId/foreshadowings',
      name: 'foreshadowing-manager',
      component: ForeshadowingManager,
    },
    {
      path: '/projects/:projectId/timeline',
      name: 'timeline-manager',
      component: TimelineManager,
    },
    {
      path: '/projects/:projectId/transfer',
      name: 'transfer-manager',
      component: TransferManager,
    },
    {
      path: '/projects/:projectId/prompts',
      name: 'prompt-settings',
      component: PromptSettings,
    },
  ],
})
