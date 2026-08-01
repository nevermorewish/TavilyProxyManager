<template>
  <n-space vertical size="large">
    <div class="page-header">
      <div class="header-info">
        <h2 class="page-title">{{ t("accessKeys.title") }}</h2>
        <div class="page-subtitle">{{ t("accessKeys.subtitle") }}</div>
      </div>
      <n-button type="primary" @click="showAdd = true">
        <template #icon><n-icon :component="AddOutline" /></template>
        {{ t("accessKeys.add") }}
      </n-button>
    </div>

    <n-alert type="info" :show-icon="true">
      {{ t("accessKeys.usage") }}
    </n-alert>

    <n-card class="table-card" :bordered="true">
      <n-data-table
        :columns="columns"
        :data="items"
        :loading="loading"
        :row-key="rowKey"
        :pagination="pagination"
        :scroll-x="840"
      />
    </n-card>

    <n-modal
      v-model:show="showAdd"
      preset="card"
      :title="t('accessKeys.addModal.title')"
      style="max-width: 480px"
      :mask-closable="!saving"
    >
      <n-form :model="addForm" label-placement="top" size="large">
        <n-form-item :label="t('accessKeys.addModal.name')" required>
          <n-input
            v-model:value="addForm.name"
            :placeholder="t('accessKeys.addModal.namePlaceholder')"
            :maxlength="100"
            autofocus
            @keyup.enter="createAccessKey"
          />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button :disabled="saving" @click="showAdd = false">
            {{ t("common.cancel") }}
          </n-button>
          <n-button
            type="primary"
            :loading="saving"
            :disabled="!addForm.name.trim()"
            @click="createAccessKey"
          >
            <template #icon><n-icon :component="AddOutline" /></template>
            {{ t("accessKeys.addModal.create") }}
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </n-space>
</template>

<script setup lang="ts">
import { h, onMounted, reactive, ref } from "vue";
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NPopconfirm,
  NSpace,
  NTooltip,
  useMessage,
  type DataTableColumns,
} from "naive-ui";
import {
  AddOutline,
  CopyOutline,
  EyeOffOutline,
  EyeOutline,
  TrashOutline,
} from "@vicons/ionicons5";
import { api } from "../api/client";
import { locale, t } from "../i18n";
import type { AccessKeyItem } from "../types";
import { writeClipboardText } from "../utils/clipboard";

const message = useMessage();
const items = ref<AccessKeyItem[]>([]);
const loading = ref(false);
const saving = ref(false);
const deletingIds = ref(new Set<number>());
const visibleIds = ref(new Set<number>());
const showAdd = ref(false);
const addForm = reactive({ name: "" });
const pagination = { pageSize: 10 };

function rowKey(row: AccessKeyItem) {
  return row.id;
}

function maskedKey(key: string): string {
  if (key.length <= 12) return "*".repeat(key.length);
  return `${key.slice(0, 6)}${"*".repeat(12)}${key.slice(-4)}`;
}

function toggleVisible(id: number): void {
  const next = new Set(visibleIds.value);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  visibleIds.value = next;
}

async function copyKey(key: string): Promise<void> {
  try {
    await writeClipboardText(key);
    message.success(t("common.copiedToClipboard"));
  } catch {
    message.error(t("common.copyFailed"));
  }
}

async function load(): Promise<void> {
  loading.value = true;
  try {
    const { data } = await api.get<AccessKeyItem[]>("/api/access-keys");
    items.value = data;
  } catch (err: any) {
    message.error(err?.response?.data?.error ?? t("accessKeys.errors.load"));
  } finally {
    loading.value = false;
  }
}

async function createAccessKey(): Promise<void> {
  const name = addForm.name.trim();
  if (!name || saving.value) return;
  saving.value = true;
  try {
    const { data } = await api.post<AccessKeyItem>("/api/access-keys", { name });
    showAdd.value = false;
    addForm.name = "";
    visibleIds.value = new Set([...visibleIds.value, data.id]);
    await load();
    message.success(t("accessKeys.messages.created"));
  } catch (err: any) {
    message.error(err?.response?.data?.error ?? t("common.createFailed"));
  } finally {
    saving.value = false;
  }
}

async function deleteAccessKey(row: AccessKeyItem): Promise<void> {
  const next = new Set(deletingIds.value);
  next.add(row.id);
  deletingIds.value = next;
  try {
    await api.delete(`/api/access-keys/${row.id}`);
    items.value = items.value.filter((item) => item.id !== row.id);
    message.success(t("accessKeys.messages.deleted"));
  } catch (err: any) {
    message.error(err?.response?.data?.error ?? t("common.deleteFailed"));
  } finally {
    const remaining = new Set(deletingIds.value);
    remaining.delete(row.id);
    deletingIds.value = remaining;
  }
}

function iconButton(icon: any, label: string, onClick: () => void, type?: "error") {
  return h(
    NTooltip,
    null,
    {
      trigger: () =>
        h(
          NButton,
          { quaternary: true, circle: true, type, onClick },
          { icon: () => h(NIcon, { component: icon }) }
        ),
      default: () => label,
    }
  );
}

const columns: DataTableColumns<AccessKeyItem> = [
  {
    title: () => t("accessKeys.table.name"),
    key: "name",
    minWidth: 160,
    render: (row) => h("span", { class: "name-cell" }, row.name),
  },
  {
    title: () => t("accessKeys.table.key"),
    key: "key",
    minWidth: 390,
    render: (row) =>
      h("div", { class: "key-cell" }, [
        h(
          "code",
          { class: "key-code" },
          visibleIds.value.has(row.id) ? row.key : maskedKey(row.key)
        ),
        iconButton(
          visibleIds.value.has(row.id) ? EyeOffOutline : EyeOutline,
          visibleIds.value.has(row.id)
            ? t("accessKeys.actions.hide")
            : t("accessKeys.actions.show"),
          () => toggleVisible(row.id)
        ),
        iconButton(CopyOutline, t("accessKeys.actions.copy"), () => copyKey(row.key)),
      ]),
  },
  {
    title: () => t("accessKeys.table.createdAt"),
    key: "created_at",
    minWidth: 180,
    render: (row) => new Date(row.created_at).toLocaleString(locale.value),
  },
  {
    title: () => t("accessKeys.table.actions"),
    key: "actions",
    width: 90,
    align: "center",
    render: (row) =>
      h(
        NPopconfirm,
        {
          onPositiveClick: () => deleteAccessKey(row),
        },
        {
          trigger: () =>
            h(
              NTooltip,
              null,
              {
                trigger: () =>
                  h(
                    NButton,
                    {
                      quaternary: true,
                      circle: true,
                      type: "error",
                      loading: deletingIds.value.has(row.id),
                    },
                    { icon: () => h(NIcon, { component: TrashOutline }) }
                  ),
                default: () => t("accessKeys.actions.delete"),
              }
            ),
          default: () => t("accessKeys.confirm.delete"),
        }
      ),
  },
];

onMounted(load);
</script>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 8px;
}

.header-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.page-title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
}

.page-subtitle {
  color: #888;
  font-size: 13px;
}

.table-card {
  border-radius: 8px;
}

.table-card :deep(.n-card__content) {
  padding: 0;
}

.name-cell {
  font-weight: 600;
}

.key-cell {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

.key-code {
  display: block;
  flex: 1;
  min-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding: 5px 8px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.05);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
}

@media (max-width: 640px) {
  .page-header {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
