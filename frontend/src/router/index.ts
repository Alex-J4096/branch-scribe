import { createRouter, createWebHistory } from 'vue-router'

import ProjectList from '@/views/ProjectList.vue'
import ProjectWorkspace from '@/views/ProjectWorkspace.vue'
import ModelProfileSettings from '@/views/ModelProfileSettings.vue'
import CanonManager from '@/views/CanonManager.vue'
import MemoryManager from '@/views/MemoryManager.vue'
import BlockTool from '@/views/BlockTool.vue'

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
      path: '/projects/:projectId/model-profiles',
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
  ],
})
