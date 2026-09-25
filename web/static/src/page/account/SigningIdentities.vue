<template>
  <section class="atv-signing-identities">
    <div class="atv-signing-identities-header">
      <div class="min-w-0">
        <h4>{{ $t("identity.title") }}</h4>
        <p class="atv-signing-identities-subtitle">{{ $t("identity.subtitle") }}</p>
      </div>
      <button type="button" class="btn btn-primary btn-sm atv-toolbar-btn" @click="openImportModal">
        {{ $t("identity.button.import") }}
      </button>
    </div>

    <div class="alert atv-warning text-sm mb-4">
      <span>{{ $t("identity.notice") }}</span>
    </div>

    <div class="overflow-x-auto">
      <table class="table w-full atv-responsive-table atv-identity-table">
        <thead>
          <tr>
            <th>{{ $t("identity.table.header.identity") }}</th>
            <th>{{ $t("identity.table.header.certificate") }}</th>
            <th>{{ $t("identity.table.header.profile") }}</th>
            <th>{{ $t("identity.table.header.validity") }}</th>
            <th>{{ $t("identity.table.header.status") }}</th>
            <th>{{ $t("home.table.header.operate") }}</th>
          </tr>
        </thead>
        <tbody class="bg-base-100">
          <tr v-if="loading" class="atv-state-row">
            <td colspan="6" class="text-center">
              <span class="loading loading-spinner loading-md"></span>
            </td>
          </tr>
          <template v-else>
            <tr v-for="identity in identities" :key="identity.id">
              <td class="atv-identity-name">
                <div class="font-bold">{{ identity.name }}</div>
                <div class="atv-identity-muted">{{ identity.certificate_common_name }}</div>
                <div class="atv-identity-muted">
                  {{ $t("identity.fields.team") }}:
                  {{ identity.team_name ? `${identity.team_name} (${identity.team_id})` : identity.team_id }}
                </div>
              </td>
              <td class="atv-field" :data-label="$t('identity.table.header.certificate')">
                <div class="atv-identity-fingerprint">
                  <span class="tooltip" :data-tip="identity.certificate_sha1">
                    <code :title="identity.certificate_sha1">SHA-1 {{ shortFingerprint(identity.certificate_sha1) }}</code>
                  </span>
                  <button
                    type="button"
                    class="btn btn-ghost btn-xs"
                    :aria-label="$t('identity.button.copy_fingerprint')"
                    @click="copyFingerprint(identity.certificate_sha1)"
                  >
                    {{ $t("identity.button.copy") }}
                  </button>
                </div>
              </td>
              <td class="atv-field" :data-label="$t('identity.table.header.profile')">
                <div class="atv-identity-stack">
                  <span class="font-semibold">{{ identity.profile_name }}</span>
                  <span class="atv-identity-muted">
                    {{ profileKindLabel(identity.profile_kind) }}
                    · {{ (identity.profile_platforms || []).join(", ") || "—" }}
                  </span>
                  <span class="atv-identity-muted">{{ deviceCountLabel(identity) }}</span>
                  <code class="atv-identity-muted">{{ identity.profile_application_identifier }}</code>
                </div>
              </td>
              <td class="atv-field" :data-label="$t('identity.table.header.validity')">
                <div class="atv-identity-stack">
                  <span class="font-semibold">
                    {{ $t("identity.fields.expires", { date: formatDate(identity.expires_at) }) }}
                  </span>
                  <span class="atv-identity-muted">
                    {{ $t("identity.fields.certificate_validity", {
                      from: formatDate(identity.certificate_not_before),
                      to: formatDate(identity.certificate_not_after),
                    }) }}
                  </span>
                  <span class="atv-identity-muted">
                    {{ $t("identity.fields.profile_validity", {
                      from: formatDate(identity.profile_creation_date),
                      to: formatDate(identity.profile_expiration_date),
                    }) }}
                  </span>
                </div>
              </td>
              <td class="atv-field" :data-label="$t('identity.table.header.status')">
                <div class="atv-identity-badges">
                  <span
                    v-for="(issue, index) in identity.status || []"
                    :key="`${issue.code}-${index}`"
                    class="atv-status"
                    :class="severityClass(issue.severity)"
                    :title="issue.message"
                  >{{ issueLabel(issue) }}</span>
                  <span v-if="identity.in_use" class="atv-status atv-status--warning">
                    {{ $t("identity.status.in_use") }}
                  </span>
                  <span class="atv-status">
                    {{ identity.app_count > 0
                      ? $t("identity.status.used_by", { count: identity.app_count })
                      : $t("identity.status.unused") }}
                  </span>
                </div>
              </td>
              <td class="atv-action-cell">
                <div class="atv-account-action-row">
                  <button
                    type="button"
                    class="btn btn-sm btn-ghost atv-account-action"
                    :disabled="identity.in_use"
                    @click="openReplaceModal(identity)"
                  >{{ $t("identity.button.replace_profile") }}</button>
                  <Popper placement="top" arrow="true">
                    <template #content="{ close }">
                      <div class="flex flex-col gap-y-2">
                        <div class="py-2">
                          {{ $t("identity.dialog.delete_confirm.title", { name: identity.name }) }}
                        </div>
                        <div class="flex gap-x-2 justify-end items-center">
                          <button type="button" class="btn btn-ghost btn-sm" @click="close">
                            {{ $t("home.dialog.delete_confirm.button.cancel") }}
                          </button>
                          <button type="button" class="btn btn-primary btn-xs" @click="deleteIdentity(identity, close)">
                            {{ $t("home.dialog.delete_confirm.button.confirm") }}
                          </button>
                        </div>
                      </div>
                    </template>
                    <button
                      type="button"
                      class="btn btn-sm btn-ghost atv-account-action atv-account-action--danger"
                      :disabled="identity.in_use"
                    >{{ $t("home.table.button.delete") }}</button>
                  </Popper>
                </div>
              </td>
            </tr>
            <tr v-if="identities.length === 0" class="atv-state-row">
              <td colspan="6" class="text-center">{{ $t("identity.table.empty") }}</td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <!-- Import Modal -->
    <dialog class="modal" :class="{ 'modal-open': importModal.visible }">
      <div class="modal-box atv-identity-modal">
        <h3 class="font-bold text-lg mb-4">{{ $t("identity.import_modal.title") }}</h3>

        <form :key="importModal.formKey" class="flex flex-col gap-y-2" autocomplete="off" @submit.prevent="submitImport">
          <template v-if="!importModal.result">
            <div class="form-control w-full">
              <label class="label" for="atv-identity-name">
                <span class="label-text">{{ $t("identity.import_modal.name_label") }}</span>
              </label>
              <input
                id="atv-identity-name"
                type="text"
                maxlength="100"
                class="input input-bordered w-full"
                :placeholder="$t('identity.import_modal.name_placeholder')"
                v-model="importModal.name"
              />
            </div>
            <div class="form-control w-full">
              <label class="label" for="atv-identity-p12">
                <span class="label-text">{{ $t("identity.import_modal.p12_label") }}</span>
              </label>
              <input
                id="atv-identity-p12"
                type="file"
                accept=".p12,.pfx,application/x-pkcs12"
                class="file-input file-input-bordered w-full"
                required
                @change="importModal.p12 = $event.target.files[0] || null"
              />
            </div>
            <div class="form-control w-full">
              <label class="label" for="atv-identity-password">
                <span class="label-text">{{ $t("identity.import_modal.password_label") }}</span>
              </label>
              <input
                id="atv-identity-password"
                type="password"
                autocomplete="new-password"
                class="input input-bordered w-full"
                :placeholder="$t('identity.import_modal.password_placeholder')"
                v-model="importModal.password"
              />
              <label class="label">
                <span class="label-text-alt whitespace-normal">{{ $t("identity.import_modal.password_tips") }}</span>
              </label>
            </div>
            <div class="form-control w-full">
              <label class="label" for="atv-identity-profile">
                <span class="label-text">{{ $t("identity.import_modal.profile_label") }}</span>
              </label>
              <input
                id="atv-identity-profile"
                type="file"
                accept=".mobileprovision,.provisionprofile"
                class="file-input file-input-bordered w-full"
                required
                @change="importModal.profile = $event.target.files[0] || null"
              />
            </div>
          </template>

          <div v-if="importModal.error" class="atv-identity-error" role="alert">
            <div class="font-semibold">{{ errorText(importModal.error) }}</div>
            <SigningIssueList v-if="importModal.error.issues.length" :issues="importModal.error.issues" />
          </div>

          <div v-if="importModal.result" class="flex flex-col gap-y-3">
            <div class="atv-status atv-status--valid self-start">
              {{ $t("identity.toast.import_success") }}: {{ importModal.result.name }}
            </div>
            <template v-if="(importModal.result.status || []).length">
              <p class="text-sm">{{ $t("identity.import_modal.issues_title") }}</p>
              <SigningIssueList :issues="importModal.result.status" />
            </template>
          </div>

          <div class="modal-action">
            <template v-if="importModal.result">
              <button type="button" class="btn btn-primary" @click="closeImportModal">
                {{ $t("identity.button.done") }}
              </button>
            </template>
            <template v-else>
              <button type="button" class="btn" :disabled="importModal.loading" @click="closeImportModal">
                {{ $t("home.dialog.delete_confirm.button.cancel") }}
              </button>
              <button type="submit" class="btn btn-primary" :disabled="importModal.loading">
                <span class="loading loading-spinner" v-show="importModal.loading"></span>
                {{ $t("identity.import_modal.submit") }}
              </button>
            </template>
          </div>
        </form>
      </div>
    </dialog>

    <!-- Replace Profile Modal -->
    <dialog class="modal" :class="{ 'modal-open': replaceModal.visible }">
      <div class="modal-box atv-identity-modal">
        <h3 class="font-bold text-lg mb-2">
          {{ $t("identity.replace_modal.title", { name: replaceModal.identity?.name || "" }) }}
        </h3>
        <p class="text-sm mb-2">
          {{ $t("identity.replace_modal.current", { name: replaceModal.identity?.profile_name || "" }) }}
        </p>
        <p class="text-sm atv-identity-muted mb-2">{{ $t("identity.replace_modal.tips") }}</p>

        <form :key="replaceModal.formKey" class="flex flex-col gap-y-2" @submit.prevent="submitReplace">
          <div v-if="!replaceModal.result" class="form-control w-full">
            <label class="label" for="atv-identity-replace-profile">
              <span class="label-text">{{ $t("identity.import_modal.profile_label") }}</span>
            </label>
            <input
              id="atv-identity-replace-profile"
              type="file"
              accept=".mobileprovision,.provisionprofile"
              class="file-input file-input-bordered w-full"
              required
              @change="replaceModal.profile = $event.target.files[0] || null"
            />
          </div>

          <div v-if="replaceModal.error" class="atv-identity-error" role="alert">
            <div class="font-semibold">{{ errorText(replaceModal.error) }}</div>
            <SigningIssueList v-if="replaceModal.error.issues.length" :issues="replaceModal.error.issues" />
          </div>

          <div v-if="replaceModal.result" class="flex flex-col gap-y-3">
            <div class="atv-status atv-status--valid self-start">
              {{ $t("identity.toast.replace_success") }}: {{ replaceModal.result.profile_name }}
            </div>
            <template v-if="(replaceModal.result.status || []).length">
              <p class="text-sm">{{ $t("identity.import_modal.issues_title") }}</p>
              <SigningIssueList :issues="replaceModal.result.status" />
            </template>
          </div>

          <div class="modal-action">
            <template v-if="replaceModal.result">
              <button type="button" class="btn btn-primary" @click="closeReplaceModal">
                {{ $t("identity.button.done") }}
              </button>
            </template>
            <template v-else>
              <button type="button" class="btn" :disabled="replaceModal.loading" @click="closeReplaceModal">
                {{ $t("home.dialog.delete_confirm.button.cancel") }}
              </button>
              <button type="submit" class="btn btn-primary" :disabled="replaceModal.loading">
                <span class="loading loading-spinner" v-show="replaceModal.loading"></span>
                {{ $t("identity.replace_modal.submit") }}
              </button>
            </template>
          </div>
        </form>
      </div>
    </dialog>

    <!-- Forced Delete Confirmation -->
    <dialog class="modal" :class="{ 'modal-open': !!referenced }">
      <div class="modal-box atv-identity-modal" v-if="referenced">
        <h3 class="font-bold text-lg mb-2">{{ $t("identity.dialog.referenced.title") }}</h3>
        <p class="text-sm">
          {{ $t("identity.dialog.referenced.content", { count: referenced.count, name: referenced.identity.name }) }}
        </p>
        <div class="modal-action">
          <button type="button" class="btn" :disabled="referenced.loading" @click="referenced = null">
            {{ $t("home.dialog.delete_confirm.button.cancel") }}
          </button>
          <button type="button" class="btn btn-error" :disabled="referenced.loading" @click="forceDelete">
            <span class="loading loading-spinner" v-show="referenced.loading"></span>
            {{ $t("identity.dialog.referenced.confirm") }}
          </button>
        </div>
      </div>
    </dialog>
  </section>
</template>

<script>
import dayjs from "dayjs";
import api from "@/api/api";
import { toast } from "vue3-toastify";
import SigningIssueList from "@/components/SigningIssueList.vue";
import {
  issueText,
  normalizeSeverity,
  requestErrorOf as requestError,
  shortFingerprint,
  signingCodeText,
} from "@/utils/signing-report.mjs";

// Mirrors signing.MaxP12Size and signing.MaxProfileSize: larger files are
// refused by the server anyway, checking early avoids a useless upload.
const maxP12Size = 1 << 20;
const maxProfileSize = 2 << 20;

function emptyImportModal(formKey = 0) {
  return {
    visible: false,
    formKey,
    name: "",
    password: "",
    p12: null,
    profile: null,
    loading: false,
    error: null,
    result: null,
  };
}

function emptyReplaceModal(formKey = 0) {
  return {
    visible: false,
    formKey,
    identity: null,
    profile: null,
    loading: false,
    error: null,
    result: null,
  };
}

export default {
  name: "SigningIdentities",
  components: { SigningIssueList },
  data() {
    return {
      identities: [],
      loading: false,
      importModal: emptyImportModal(),
      replaceModal: emptyReplaceModal(),
      referenced: null,
    };
  },
  created() {
    this.fetchIdentities().then(() => {
      // The install page links here with ?section=signing-identities.
      if (this.$route.query.section === "signing-identities") {
        this.$nextTick(() => this.$el.scrollIntoView({ block: "start" }));
      }
    });
  },
  methods: {
    shortFingerprint,
    fetchIdentities() {
      this.loading = true;
      return api
        .getSigningIdentities()
        .then((res) => {
          this.identities = res.data || [];
        })
        .catch((err) => {
          this.toastRequestError(err);
        })
        .finally(() => {
          this.loading = false;
        });
    },
    translate(key) {
      return this.$t(key);
    },
    errorText(error) {
      return signingCodeText(error?.code, error?.message, this.translate);
    },
    // toastRequestError reports API failures; transport failures were
    // already reported by utils/request.js.
    toastRequestError(err) {
      if (err?.result) {
        toast.error(this.errorText(requestError(err)));
      }
    },
    issueLabel(issue) {
      return issueText(issue, this.translate);
    },
    severityClass(severity) {
      switch (normalizeSeverity(severity)) {
        case "error":
          return "atv-status--invalid";
        case "warning":
          return "atv-status--warning";
        default:
          return "";
      }
    },
    profileKindLabel(kind) {
      const key = `signing.profile_kinds.${kind}`;
      const text = this.$t(key);
      return kind && text !== key ? text : kind || "—";
    },
    deviceCountLabel(identity) {
      if (identity.profile_provisions_all_devices) {
        return this.$t("identity.fields.all_devices");
      }
      return this.$t("identity.fields.devices", { count: identity.profile_device_count || 0 });
    },
    formatDate(value) {
      const date = dayjs(value);
      if (!value || !date.isValid() || date.year() <= 1) {
        return "—";
      }
      return date.format("YYYY-MM-DD");
    },
    async copyFingerprint(value) {
      try {
        if (navigator.clipboard && window.isSecureContext) {
          await navigator.clipboard.writeText(value);
        } else {
          // Plain-HTTP deployments have no Clipboard API.
          const input = document.createElement("textarea");
          input.value = value;
          input.setAttribute("readonly", "");
          input.style.position = "fixed";
          input.style.opacity = "0";
          document.body.appendChild(input);
          input.select();
          const copied = document.execCommand("copy");
          document.body.removeChild(input);
          if (!copied) {
            throw new Error("copy command rejected");
          }
        }
        toast.success(this.$t("identity.toast.copied"));
      } catch {
        toast.error(this.$t("identity.toast.copy_failed"));
      }
    },

    openImportModal() {
      this.importModal = { ...emptyImportModal(this.importModal.formKey + 1), visible: true };
    },
    closeImportModal() {
      // Drop the password and the selected files together with the form.
      this.importModal = emptyImportModal(this.importModal.formKey + 1);
    },
    async submitImport() {
      const modal = this.importModal;
      if (!modal.p12 || !modal.profile) {
        modal.error = { code: "", message: this.$t("identity.import_modal.files_required"), issues: [] };
        return;
      }
      if (modal.p12.size > maxP12Size || modal.profile.size > maxProfileSize) {
        modal.error = { code: "upload_too_large", message: "", issues: [] };
        return;
      }

      const formData = new FormData();
      formData.append("name", modal.name.trim());
      formData.append("password", modal.password);
      formData.append("p12", modal.p12);
      formData.append("profile", modal.profile);

      modal.loading = true;
      modal.error = null;
      try {
        const res = await api.importSigningIdentity(formData);
        modal.password = "";
        modal.result = res.data || {};
        toast.success(this.$t("identity.toast.import_success"));
        this.fetchIdentities();
        // Informational notes alone, such as the unchecked revocation that
        // the page notice already shows, need no findings screen.
        if (!(modal.result.status || []).some((issue) => normalizeSeverity(issue?.severity) !== "info")) {
          this.closeImportModal();
        }
      } catch (err) {
        modal.error = requestError(err);
      } finally {
        modal.loading = false;
      }
    },

    openReplaceModal(identity) {
      this.replaceModal = {
        ...emptyReplaceModal(this.replaceModal.formKey + 1),
        visible: true,
        identity,
      };
    },
    closeReplaceModal() {
      this.replaceModal = emptyReplaceModal(this.replaceModal.formKey + 1);
    },
    async submitReplace() {
      const modal = this.replaceModal;
      if (!modal.profile) {
        return;
      }
      if (modal.profile.size > maxProfileSize) {
        modal.error = { code: "upload_too_large", message: "", issues: [] };
        return;
      }

      modal.loading = true;
      modal.error = null;
      try {
        const res = await api.replaceSigningIdentityProfile(modal.identity.id, modal.profile);
        modal.result = res.data || {};
        toast.success(this.$t("identity.toast.replace_success"));
        this.fetchIdentities();
        if (!(modal.result.status || []).some((issue) => normalizeSeverity(issue?.severity) !== "info")) {
          this.closeReplaceModal();
        }
      } catch (err) {
        modal.error = requestError(err);
      } finally {
        modal.loading = false;
      }
    },

    async deleteIdentity(identity, close) {
      close?.();
      try {
        await api.deleteSigningIdentity(identity.id, false);
        this.onDeleted();
      } catch (err) {
        const error = requestError(err);
        if (error.code === "identity_referenced") {
          this.referenced = {
            identity,
            count: error.appCount || identity.app_count || 0,
            loading: false,
          };
          return;
        }
        this.toastRequestError(err);
      }
    },
    async forceDelete() {
      const referenced = this.referenced;
      referenced.loading = true;
      try {
        await api.deleteSigningIdentity(referenced.identity.id, true);
        this.referenced = null;
        this.onDeleted();
      } catch (err) {
        referenced.loading = false;
        this.referenced = null;
        this.toastRequestError(err);
      }
    },
    onDeleted() {
      toast.success(this.$t("identity.toast.delete_success"));
      this.fetchIdentities();
    },
  },
};
</script>

<style scoped>
.atv-signing-identities {
  margin-top: 40px;
}

.atv-signing-identities-header {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  padding-inline: 4px;
}

.atv-signing-identities-header h4 {
  margin: 0 0 4px;
}

.atv-signing-identities-subtitle,
.atv-identity-muted {
  color: var(--atv-muted);
  font-size: .8rem;
  line-height: 1.45;
}

.atv-identity-stack {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  overflow-wrap: anywhere;
}

.atv-identity-fingerprint {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.atv-identity-fingerprint code {
  font-size: .78rem;
  white-space: nowrap;
}

.atv-identity-fingerprint .btn-xs {
  min-height: 28px;
  height: 28px;
  padding-inline: 8px;
  font-size: .72rem;
}

.atv-identity-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.atv-identity-badges .atv-status {
  white-space: normal;
}

.atv-identity-error {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border: 1px solid var(--atv-danger);
  border-radius: 12px;
  background: var(--atv-danger-soft);
  color: var(--atv-danger);
  font-size: .88rem;
}

.app-container .modal-box.atv-identity-modal {
  width: min(560px, calc(100vw - 24px));
  max-width: 560px;
  max-height: calc(100dvh - 48px);
  overflow: auto;
}

@media (max-width: 767px) {
  .atv-signing-identities {
    margin-top: 28px;
  }

  .app-container .modal-box.atv-identity-modal {
    padding: 16px;
    max-height: calc(100dvh - 24px);
  }

  /* Identity fields hold several lines and long identifiers: put the label
     above the value instead of squeezing it into a side column. */
  .app-container .atv-account-page .atv-identity-table.atv-responsive-table tbody td.atv-field {
    grid-template-columns: minmax(0, 1fr);
    gap: 4px;
  }

  .app-container .atv-account-page .atv-identity-table.atv-responsive-table tbody td.atv-field > * {
    justify-self: start;
  }
}
</style>
