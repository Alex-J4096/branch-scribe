<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ArrowLeft, Bot, KeyRound, Plus, Save, SlidersHorizontal, Trash2 } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'

import { api } from '@/api/client'
import type { ModelProfile, ModelProfileInput } from '@/api/types'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()

const projectId = computed(() => String(route.params.projectId))
const selectedProfileId = ref<string | null>(null)

const providerBaseUrls: Partial<Record<ModelProfileInput['provider'], string>> = {
  openai: 'https://api.openai.com/v1',
  openrouter: 'https://openrouter.ai/api/v1',
  deepseek: 'https://api.deepseek.com/v1',
  moonshot: 'https://api.moonshot.cn/v1',
  siliconflow: 'https://api.siliconflow.cn/v1',
}

const form = reactive<ModelProfileInput>({
  name: '',
  provider: 'openai_compatible',
  model: '',
  base_url: '',
  api_key: 'BRANCHSCRIBE_MODEL_API_KEY',
  temperature: 0.8,
  top_p: 0.9,
  max_tokens: 2048,
  context_window: 32768,
})

const projectQuery = useQuery({
  queryKey: computed(() => ['project', projectId.value]),
  queryFn: () => api.getProject(projectId.value),
})

const profilesQuery = useQuery({
  queryKey: computed(() => ['model-profiles', projectId.value]),
  queryFn: () => api.listModelProfiles(projectId.value),
})

const profiles = computed(() => profilesQuery.data.value ?? [])
const selectedProfile = computed(() => profiles.value.find((profile) => profile.id === selectedProfileId.value) ?? null)
const providerDefaultBaseUrl = computed(() => providerBaseUrls[form.provider] ?? '')

watch(
  profiles,
  (value) => {
    if (!selectedProfileId.value && value[0]) {
      selectProfile(value[0])
    }
  },
  { immediate: true },
)

const createProfile = useMutation({
  mutationFn: () => api.createModelProfile(projectId.value, sanitizeForm()),
  onSuccess: async (profile) => {
    selectedProfileId.value = profile.id
    form.api_key = ''
    await queryClient.invalidateQueries({ queryKey: ['model-profiles', projectId.value] })
  },
})

const updateProfile = useMutation({
  mutationFn: () => {
    if (!selectedProfileId.value) throw new Error('No selected model profile')
    return api.updateModelProfile(selectedProfileId.value, sanitizeForm())
  },
  onSuccess: async () => {
    form.api_key = ''
    await queryClient.invalidateQueries({ queryKey: ['model-profiles', projectId.value] })
  },
})

const deleteProfile = useMutation({
  mutationFn: api.deleteModelProfile,
  onSuccess: async (_result, profileId) => {
    if (selectedProfileId.value === profileId) {
      selectedProfileId.value = null
      resetForm()
    }
    await queryClient.invalidateQueries({ queryKey: ['model-profiles', projectId.value] })
  },
})

function selectProfile(profile: ModelProfile) {
  selectedProfileId.value = profile.id
  form.name = profile.name
  form.provider = profile.provider
  form.model = profile.model
  form.base_url = profile.base_url ?? ''
  form.api_key = ''
  form.temperature = profile.temperature
  form.top_p = profile.top_p
  form.max_tokens = profile.max_tokens
  form.context_window = profile.context_window
}

function resetForm() {
  selectedProfileId.value = null
  form.name = ''
  form.provider = 'openai_compatible'
  form.model = ''
  form.base_url = ''
  form.api_key = 'BRANCHSCRIBE_MODEL_API_KEY'
  form.temperature = 0.8
  form.top_p = 0.9
  form.max_tokens = 2048
  form.context_window = 32768
}

watch(
  () => form.provider,
  (provider, previousProvider) => {
    const currentBaseUrl = form.base_url?.trim() ?? ''
    const previousDefault = previousProvider ? providerBaseUrls[previousProvider] : ''
    if (currentBaseUrl && currentBaseUrl !== previousDefault) return
    form.base_url = providerBaseUrls[provider] ?? ''
  },
)

function submitProfile() {
  if (!form.name.trim() || !form.model.trim()) return
  if (selectedProfileId.value) {
    updateProfile.mutate()
  } else {
    createProfile.mutate()
  }
}

function sanitizeForm(): ModelProfileInput {
  return {
    name: form.name.trim(),
    provider: form.provider,
    model: form.model.trim(),
    base_url: form.base_url?.trim() || null,
    api_key: form.api_key?.trim() || null,
    temperature: Number(form.temperature),
    top_p: Number(form.top_p),
    max_tokens: Number(form.max_tokens),
    context_window: Number(form.context_window),
  }
}
</script>

<template>
  <main class="settings-page">
    <header class="workspace__topbar">
      <button class="icon-button" type="button" title="返回工作台" @click="router.push({ name: 'workspace', params: { projectId } })">
        <ArrowLeft :size="18" aria-hidden="true" />
      </button>
      <div class="workspace__title">
        <strong>{{ projectQuery.data.value?.name ?? 'BranchScribe' }}</strong>
        <span>模型配置</span>
      </div>
      <button class="button" type="button" @click="resetForm">
        <Plus :size="16" aria-hidden="true" />
        新建
      </button>
    </header>

    <section class="settings-page__body">
      <aside class="settings-list">
        <div class="settings-list__header">
          <span>Profiles</span>
          <button class="icon-button" type="button" title="新建模型" @click="resetForm">
            <Plus :size="15" aria-hidden="true" />
          </button>
        </div>
        <div v-if="profilesQuery.isLoading.value" class="empty-state empty-state--compact">正在加载模型</div>
        <div v-else-if="profiles.length === 0" class="empty-state empty-state--compact">暂无模型</div>
        <button
          v-for="profile in profiles"
          :key="profile.id"
          class="settings-list__item"
          :class="{ 'is-active': profile.id === selectedProfileId }"
          type="button"
          @click="selectProfile(profile)"
        >
          <strong>{{ profile.name }}</strong>
          <span>{{ profile.model }}</span>
          <small>
            <em>{{ profile.provider }}</em>
            <b :class="{ 'is-ready': profile.has_api_key }">{{ profile.has_api_key ? 'Key' : 'No key' }}</b>
          </small>
        </button>
      </aside>

      <form class="settings-form" @submit.prevent="submitProfile">
        <div class="settings-form__header">
          <div>
            <h1>{{ selectedProfile ? form.name || '编辑模型' : '新建模型' }}</h1>
            <p>{{ form.provider }} · {{ form.model || 'model' }}</p>
          </div>
          <Bot :size="22" aria-hidden="true" />
        </div>

        <section class="settings-group">
          <div class="settings-group__header">
            <h2>Provider</h2>
            <KeyRound :size="17" aria-hidden="true" />
          </div>
          <div class="settings-form__grid">
            <label>
              <span>名称</span>
              <input v-model="form.name" type="text" placeholder="模型名称" />
            </label>
            <label>
              <span>Provider</span>
              <select v-model="form.provider">
                <option value="openai_compatible">OpenAI-compatible</option>
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
                <option value="gemini">Gemini</option>
                <option value="openrouter">OpenRouter</option>
                <option value="deepseek">DeepSeek</option>
                <option value="moonshot">Moonshot</option>
                <option value="siliconflow">SiliconFlow</option>
              </select>
            </label>
            <label>
              <span>Base URL</span>
              <input v-model="form.base_url" type="url" :placeholder="providerDefaultBaseUrl || 'https://api.openai.com/v1'" />
            </label>
            <label>
              <span>Model</span>
              <input v-model="form.model" type="text" placeholder="gpt-4.1-mini" />
            </label>
            <label class="settings-form__wide">
              <span>API key 环境变量 · {{ selectedProfile?.has_api_key ? '已配置' : '未配置' }}</span>
              <input v-model="form.api_key" type="text" placeholder="例如 BRANCHSCRIBE_MODEL_API_KEY；编辑已有配置时留空则不修改" autocomplete="off" />
            </label>
          </div>
        </section>

        <section class="settings-group">
          <div class="settings-group__header">
            <h2>Generation</h2>
            <SlidersHorizontal :size="17" aria-hidden="true" />
          </div>
          <div class="settings-params">
            <label>
              <span>Temperature</span>
              <input v-model.number="form.temperature" type="range" min="0" max="2" step="0.1" />
              <input v-model.number="form.temperature" type="number" min="0" max="2" step="0.1" />
            </label>
            <label>
              <span>Top P</span>
              <input v-model.number="form.top_p" type="range" min="0" max="1" step="0.05" />
              <input v-model.number="form.top_p" type="number" min="0" max="1" step="0.05" />
            </label>
            <label>
              <span>Max tokens</span>
              <input v-model.number="form.max_tokens" type="range" min="256" max="8192" step="128" />
              <input v-model.number="form.max_tokens" type="number" min="1" step="128" />
            </label>
            <label>
              <span>Context window</span>
              <input v-model.number="form.context_window" type="range" min="4096" max="200000" step="1024" />
              <input v-model.number="form.context_window" type="number" min="1" step="1024" />
            </label>
          </div>
        </section>

        <footer class="settings-form__actions">
          <button class="button button--primary" type="submit" :disabled="createProfile.isPending.value || updateProfile.isPending.value">
            <Save :size="16" aria-hidden="true" />
            保存
          </button>
          <button
            v-if="selectedProfile"
            class="button"
            type="button"
            :disabled="deleteProfile.isPending.value"
            @click="deleteProfile.mutate(selectedProfile.id)"
          >
            <Trash2 :size="16" aria-hidden="true" />
            删除
          </button>
        </footer>
      </form>
    </section>
  </main>
</template>
