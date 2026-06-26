import { createRouter, createWebHistory } from 'vue-router'

import ProjectList from '@/views/ProjectList.vue'
import ProjectWorkspace from '@/views/ProjectWorkspace.vue'
import ModelProfileSettings from '@/views/ModelProfileSettings.vue'

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
      path: '/projects/:projectId/model-profiles',
      name: 'model-profiles',
      component: ModelProfileSettings,
    },
  ],
})
