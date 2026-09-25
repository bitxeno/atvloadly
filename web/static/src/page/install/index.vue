<template>
  <div class="max-w-screen-md mx-auto flex flex-col gap-y-6 atv-install-page">
    <div class="alert atv-warning" v-if="!isExternal">
      <div class="w-8">
        <WarningIcon />
      </div>
      <span class="text-sm">{{ $t("install.tips.warning") }}</span>
    </div>
    <div class="alert atv-warning" v-else>
      <div class="w-8">
        <WarningIcon />
      </div>
      <span class="text-sm">{{ $t("install.tips.external") }}</span>
    </div>

    <div class="card atv-install-card">
      <div class="lg:flex lg:flex-row">
        <div class="flex flex-col justify-center place-items-center gap-y-4 atv-install-device">
          <div class="relative w-32 rounded atv-install-device-icon">
            <IPhoneIcon v-if="isIPhone(device)" />
            <AppleTVIcon v-else />
            <button
              v-if="canShowScreenshotAction(device)"
              class="btn btn-circle btn-sm atv-install-screenshot-btn"
              @click="openScreenshotDialog"
              :disabled="screenshot.loading || !device.id"
              :title="$t('install.screenshot.action')"
              :aria-label="$t('install.screenshot.action')"
            >
              <span class="w-4 h-4"><CameraIcon /></span>
            </button>
          </div>
          <div class="flex flex-col gap-y-2 items-center justify-center">
            <span>{{ device.name }}</span>
            <span>({{ truncateIP(device.ip) }})</span>
          </div>
        </div>

        <div class="divider divider-horizontal"></div>

        <div class="p-6 flex flex-col gap-y-4 w-full max-w-lg atv-install-form-wrap">
          <form id="form" class="flex flex-col gap-y-4" @submit.prevent>
            <div class="form-control w-full">
              <label class="label">
                <span class="label-text">{{ $t("install.form.signing_mode.label") }}</span>
              </label>
              <div
                class="join w-full atv-install-signing-mode"
                role="radiogroup"
                :aria-label="$t('install.form.signing_mode.label')"
              >
                <button
                  v-for="mode in signingModes"
                  :key="mode"
                  type="button"
                  role="radio"
                  class="btn join-item flex-1"
                  :class="{ 'btn-primary': signing.mode === mode }"
                  :aria-checked="signing.mode === mode"
                  :disabled="loading"
                  @click="setSigningMode(mode)"
                >
                  {{ $t(`signing.mode.${mode}`) }}
                </button>
              </div>
            </div>

            <div class="join w-full atv-install-mode">
              <button
                type="button"
                class="btn join-item flex-1 gap-x-2"
                :class="{ 'btn-primary': installMode === 'file' }"
                :aria-pressed="installMode === 'file'"
                :aria-label="$t('install.form.mode.file')"
                :title="$t('install.form.mode.file')"
                @click="setInstallMode('file')"
              >
                <span class="w-6 h-6"><FolderOpenIcon /></span>
                <span class="hidden md:inline">{{ $t("install.form.mode.file") }}</span>
              </button>
              <!-- External signing checks an uploaded IPA before installing: file mode only. -->
              <button
                type="button"
                class="btn join-item flex-1 gap-x-2"
                :class="{ 'btn-primary': installMode === 'link' }"
                :aria-pressed="installMode === 'link'"
                :aria-label="$t('install.form.mode.link')"
                :title="isExternal ? $t('install.form.ipa_url.external_unavailable') : $t('install.form.mode.link')"
                :disabled="isExternal"
                @click="setInstallMode('link')"
              >
                <span class="w-6 h-6"><LinkIcon /></span>
                <span class="hidden md:inline">{{ $t("install.form.mode.link") }}</span>
              </button>
              <button
                type="button"
                class="btn join-item flex-1 gap-x-2"
                :class="{ 'btn-primary': installMode === 'source' }"
                :aria-pressed="installMode === 'source'"
                :aria-label="$t('install.form.mode.source')"
                :title="isExternal ? $t('install.form.ipa_url.external_unavailable') : $t('install.form.mode.source')"
                :disabled="isExternal"
                @click="setInstallMode('source')"
              >
                <span class="w-6 h-6"><GithubIcon /></span>
                <span class="hidden md:inline">{{ $t("install.form.mode.source") }}</span>
              </button>
            </div>

            <SourcePicker
              v-if="installMode === 'source'"
              :device-class="device.device_class"
              :initial="sourceInitial"
              @update:selection="onSourceSelection"
            />
            <div class="form-control w-full" v-else>
              <label class="label">
                <span class="label-text">
                  <template v-if="installMode === 'file'">{{ $t("install.form.choose_ipa.label") }}</template>
                  <template v-else>{{ $t("install.form.ipa_url.label") }}</template>
                </span>
              </label>
              <input
                v-if="installMode === 'file'"
                type="file"
                class="file-input file-input-bordered w-full"
                @change="onFileChange"
                accept=".ipa,.tipa"
                required
              />
              <input
                v-else
                type="url"
                class="input input-bordered w-full"
                v-model="ipaUrl"
                placeholder="https://example.com/app.ipa"
                required
                @input="onIpaUrlInput"
                @change="switchToSourceIfRepo"
              />
            </div>

            <div class="form-control w-full" v-if="!isExternal">
              <label class="label">
                <span class="label-text">{{
                  $t("install.form.account.label")
                }}</span>
              </label>
              <div class="join w-full atv-install-account-picker">
                <select
                  class="select select-bordered join-item flex-1 min-w-0"
                  v-model="form.account"
                  required
                >
                  <option value="" disabled selected>
                    {{ $t("install.form.account.select.placeholder") }}
                  </option>
                  <option
                    v-for="account in accounts"
                    :key="account.email"
                    :value="account.email"
                  >
                    {{ account.email }}
                    {{
                      account.email === recommendedAccount
                        ? "(" + $t("install.form.account.last_used") + ")"
                        : "(" + accountStatusLabel(account.status) + ")"
                    }}
                  </option>
                </select>
                <button type="button" class="btn join-item" @click.prevent="showLoginDialog">
                  <div class="w-6 h-6">
                    <PersonIcon />
                  </div>
                </button>
              </div>
              <label class="label">
                <span class="label-text-alt block whitespace-normal break-words">{{
                  $t("install.form.account.alt")
                }}</span>
              </label>
            </div>

            <div class="form-control w-full" v-if="isExternal">
              <label class="label">
                <span class="label-text">{{ $t("install.form.identity.label") }}</span>
              </label>
              <div class="join w-full atv-install-account-picker">
                <select
                  class="select select-bordered join-item flex-1 min-w-0"
                  v-model="signing.identityId"
                  required
                >
                  <option value="" disabled>
                    {{ $t("install.form.identity.placeholder") }}
                  </option>
                  <option
                    v-for="identity in signing.identities"
                    :key="identity.id"
                    :value="identity.id"
                  >
                    {{ identity.name }}
                    ({{ identity.id === recommendedIdentityId
                      ? $t("install.form.account.last_used")
                      : $t("install.form.identity.expires", { date: formatDate(identity.expires_at) }) }})
                  </option>
                </select>
                <button
                  type="button"
                  class="btn join-item"
                  :title="$t('install.form.identity.manage')"
                  :aria-label="$t('install.form.identity.manage')"
                  @click.prevent="manageIdentities"
                >
                  <div class="w-6 h-6">
                    <SettingsIcon />
                  </div>
                </button>
              </div>
              <label class="label">
                <span class="label-text-alt block whitespace-normal break-words">{{
                  signing.identities.length > 0
                    ? $t("install.form.identity.alt")
                    : $t("install.form.identity.empty")
                }}</span>
              </label>
            </div>

            <div class="form-control">
              <label class="label">
                <span class="label-text">{{ $t("install.form.custom_name.label") }}</span>
                <div class="tooltip" :data-tip="$t('install.form.custom_name.tips')">
                  <div class="w-4 h-4 text-secondary-content"><HelpIcon /></div>
                </div>
              </label>
              <input
                type="text"
                class="input input-bordered w-full"
                maxlength="64"
                :placeholder="$t('install.form.custom_name.placeholder')"
                v-model="form.custom_name"
              />
            </div>

            <div class="form-control" v-if="isExternal">
              <label class="label" for="install-custom-identifier">
                <span class="label-text">{{ $t("install.form.custom_identifier.label") }}</span>
              </label>
              <input
                id="install-custom-identifier"
                type="text"
                class="input input-bordered w-full"
                maxlength="255"
                autocapitalize="off"
                autocomplete="off"
                spellcheck="false"
                :disabled="loading"
                :placeholder="customIdentifierPlaceholder"
                v-model.lazy.trim="form.custom_identifier"
              />
              <label class="label" for="install-custom-identifier">
                <span class="label-text-alt block whitespace-normal break-words">{{
                  $t("install.form.custom_identifier.alt")
                }}</span>
              </label>
            </div>

            <div class="form-control">
              <label class="label cursor-pointer justify-between items-center gap-x-4">
                <div class="flex items-center">
                  <span class="label-text">{{
                    $t("install.form.extensions.remove_extensions")
                  }}</span>
                  <div class="tooltip" :data-tip="isExternal
                    ? $t('install.form.extensions.tips_external')
                    : $t('install.form.extensions.tips')">
                    <div class="w-4 h-4 text-secondary-content"><HelpIcon /></div>
                  </div>
                </div>
                <input
                  type="checkbox"
                  class="toggle toggle-success"
                  v-model="form.remove_extensions"
                />
              </label>
            </div>

            <div class="form-control" v-if="showAllowMissingEntitlements">
              <label class="label cursor-pointer justify-between items-center gap-x-4">
                <div class="flex items-center">
                  <span class="label-text">{{
                    $t("install.form.allow_missing_entitlements.label")
                  }}</span>
                  <div class="tooltip" :data-tip="$t('install.form.allow_missing_entitlements.tips')">
                    <div class="w-4 h-4 text-secondary-content"><HelpIcon /></div>
                  </div>
                </div>
                <input
                  type="checkbox"
                  class="toggle toggle-warning"
                  :disabled="loading"
                  v-model="signing.allowMissingEntitlements"
                />
              </label>
            </div>

            <div class="form-control" v-if="installMode === 'source'">
              <label class="label cursor-pointer justify-between items-center gap-x-4">
                <div class="flex items-center">
                  <span class="label-text">{{
                    $t("install.form.source.auto_update")
                  }}</span>
                  <div class="tooltip" :data-tip="$t('install.form.source.auto_update_tips')">
                    <div class="w-4 h-4 text-secondary-content"><HelpIcon /></div>
                  </div>
                </div>
                <input
                  type="checkbox"
                  class="toggle toggle-success"
                  v-model="form.auto_update"
                />
              </label>
            </div>

          </form>

          <section v-if="isExternal" class="atv-install-plan" aria-live="polite">
            <div class="atv-install-plan-header">
              <h5>{{ $t("signing.plan.title") }}</h5>
              <button
                v-if="canRerunCheck"
                type="button"
                class="btn btn-sm btn-ghost"
                @click="rerunCheck"
              >
                {{ $t("signing.plan.run_check") }}
              </button>
            </div>
            <div v-if="signing.uploading" class="atv-install-plan-progress">
              <span class="loading loading-spinner loading-sm"></span>
              {{ $t("signing.plan.uploading") }}
            </div>
            <div v-else-if="signing.checking" class="atv-install-plan-progress">
              <span class="loading loading-spinner loading-sm"></span>
              {{ $t("signing.plan.checking") }}
            </div>
            <div v-else-if="signing.checkError" class="atv-install-plan-error" role="alert">
              <div class="font-semibold">{{ $t("signing.plan.check_failed") }}</div>
              <div>{{ codeText(signing.checkError.code, signing.checkError.message) }}</div>
              <SigningIssueList
                v-if="signing.checkError.issues.length"
                :issues="signing.checkError.issues"
              />
            </div>
            <SigningPlan v-else-if="checkSummary" :summary="checkSummary" />
            <p v-else class="atv-install-plan-hint">{{ $t(planHintKey) }}</p>
          </section>

          <div class="flex flex-row gap-x-4">
            <button class="btn flex-1" @click="goBack">
              {{ $t("install.form.button.back") }}
            </button>
            <button
              class="btn btn-primary flex-1"
              @click="onSubmit"
              :disabled="loading || !canInstall"
            >
              <span class="loading loading-spinner" v-show="loading"></span
              >{{ $t("install.form.button.submit") }}
            </button>
          </div>
        </div>
      </div>

      <Login ref="loginModal" @success="fetchData" />
    </div>

    <section
      v-if="report.plan || report.failure || report.verified"
      class="card atv-install-report"
      aria-live="polite"
    >
      <h5>{{ $t("signing.report.title") }}</h5>
      <div v-if="report.failure" class="atv-install-plan-error" role="alert">
        <span v-if="report.failure.class" class="atv-status atv-status--invalid self-start">
          {{ classText(report.failure.class) }}
        </span>
        <div class="font-semibold">
          {{ codeText(report.failure.code, report.failure.message) }}
        </div>
        <SigningIssueList v-if="report.failure.issues.length" :issues="report.failure.issues" />
      </div>
      <div v-if="report.verified" class="atv-status atv-status--valid self-start">
        {{ $t("signing.report.verified") }}
      </div>
      <details v-if="report.plan" :open="report.plan.blocking">
        <summary>{{ $t("signing.report.plan") }}</summary>
        <SigningPlan class="mt-3" :summary="report.plan" />
      </details>
    </section>

    <div v-show="log.show">
      <textarea
        id="log"
        class="textarea textarea-bordered w-full h-48 atv-install-log leading-5"
        wrap="off"
        v-model="log.output"
      ></textarea>
    </div>

    <!-- Screenshot Modal -->
    <dialog
      ref="screenshotModal"
      :class="['modal', { 'modal-open': screenshot.visible }]"
    >
      <div class="modal-box max-w-3xl">
        <button class="btn btn-sm btn-circle btn-ghost absolute right-2 top-2" @click="closeScreenshotDialog">✕</button>
        <h3 class="font-bold text-lg flex items-center gap-x-2">
          <span class="w-5 h-5"><CameraIcon /></span>
          {{ $t("install.screenshot.title") }}
        </h3>

        <div class="mt-4 flex flex-col gap-y-3">
          <div
            class="flex flex-row items-center gap-x-2 text-sm"
            v-show="screenshot.status"
          >
            <span
              class="loading loading-spinner loading-xs"
              v-show="screenshot.loading"
            ></span>
            <span class="text-base-content/70">{{ screenshot.status }}</span>
          </div>

          <div
            class="flex items-center justify-center bg-neutral rounded overflow-hidden mx-auto w-full max-w-[640px] aspect-[16/9]"
            :class="{ 'p-2': !screenshot.image }"
          >
            <img
              v-if="screenshot.image"
              :src="screenshot.image"
              :alt="$t('install.screenshot.title')"
              class="w-full h-full object-contain"
            />
            <span
              v-else-if="!screenshot.loading"
              class="text-base-100/70 text-sm"
            >
              {{ $t("install.screenshot.empty") }}
            </span>
          </div>
        </div>

        <div class="modal-action">
          <button
            class="btn gap-x-2"
            @click="downloadScreenshot"
            :disabled="!screenshot.image || screenshot.loading"
          >
            <span class="w-4 h-4"><DownloadIcon /></span>
            <span>{{ $t("install.screenshot.button.download") }}</span>
          </button>
          <button
            class="btn btn-primary gap-x-2"
            @click="captureScreenshot"
            :disabled="screenshot.loading"
          >
            <span class="w-4 h-4"><RefreshIcon /></span>
            <span>{{ $t("install.screenshot.button.refresh") }}</span>
          </button>
        </div>
      </div>
    </dialog>
  </div>
</template>
  
  <script>
import api from "@/api/api";
import dayjs from "dayjs";
import { toast } from "vue3-toastify";
import { parseBundleIdFromPlist } from "@/utils/utils";
import { installFailureMessage as formatInstallFailureMessage } from "@/utils/install-error-feedback.mjs";
import { accountStatusLabel as formatAccountStatus } from "@/utils/install-feedback.mjs";
import {
  createSigningReportStream,
  hasWaivableEntitlementIssues,
  requestErrorOf,
  signingCodeText,
  summarizePlan,
} from "@/utils/signing-report.mjs";
import { guessSourceKind } from "@/utils/source.mjs";
import JSZip from "jszip";
import Login from "@/components/Login.vue";
import SigningIssueList from "@/components/SigningIssueList.vue";
import SigningPlan from "@/components/SigningPlan.vue";
import SourcePicker from "@/components/SourcePicker.vue";

const appleIDMode = "apple_id";
const externalMode = "external_certificate";

export default {
  components: { Login, SigningIssueList, SigningPlan, SourcePicker },
  data() {
    return {
      id: "",
      installMode: "file",
      ipaUrl: "",
      files: [],
      // Source to prefill when a repository URL switched the page to source mode.
      sourceInitial: null,
      sourceSelection: null,
      ipa: {},
      device: {},
      loading: false,
      accounts: [],
      installedApps: [],
      recommendedAccount: "",
      recommendedIdentityId: 0,
      signingModes: [appleIDMode, externalMode],
      signing: {
        mode: appleIDMode,
        // Set once the user picks a mode; stops the automatic default.
        modeChosen: false,
        identities: [],
        identityId: "",
        allowMissingEntitlements: false,
        // IPA uploaded ahead of time so the compatibility check can read it.
        uploaded: null,
        uploading: false,
        checking: false,
        check: null,
        checkError: null,
      },
      // Structured SIGNING_REPORT stages of the running installation.
      report: {
        plan: null,
        failure: null,
        verified: false,
      },
      // Signing mode of the running installation.
      submittedMode: "",
      form: {
        account: "",
        password: "",
        custom_name: "",
        remove_extensions: false,
        // Main bundle identifier of the signed app (external mode); empty
        // keeps the identifiers of the IPA. Synced on change, not on input,
        // so that the compatibility check runs once per edit.
        custom_identifier: "",
        auto_update: false,
      },
      log: {
        newcontent : "",
        output: "",
        show: false,
      },

      refreshLogInterval: null,

      screenshot: {
        visible: false,
        loading: false,
        image: "",
        status: "",
      },
    };
  },
  computed: {
    isExternal() {
      return this.signing.mode === externalMode;
    },
    checkSummary() {
      const check = this.signing.check;
      return check ? summarizePlan(check.plan, check.blocking) : null;
    },
    customIdentifierPlaceholder() {
      return this.signing.uploaded?.bundle_identifier ||
        this.signing.check?.plan?.main_bundle_id ||
        this.$t("install.form.custom_identifier.placeholder");
    },
    showAllowMissingEntitlements() {
      return this.isExternal && (
        this.signing.allowMissingEntitlements ||
        hasWaivableEntitlementIssues(this.signing.check?.plan?.issues)
      );
    },
    canInstall() {
      if (!this.isExternal) {
        return true;
      }
      const signing = this.signing;
      return !!(
        signing.uploaded &&
        signing.identityId &&
        this.checkSummary &&
        !this.checkSummary.blocking &&
        !signing.uploading &&
        !signing.checking
      );
    },
    canRerunCheck() {
      const signing = this.signing;
      return this.isExternal && !this.loading && !signing.uploading && !signing.checking &&
        this.files.length > 0 && !!signing.identityId;
    },
    planHintKey() {
      if (this.files.length > 0 && this.signing.identityId && !this.signing.uploaded) {
        return "signing.plan.rerun_needed";
      }
      return "signing.plan.pending";
    },
  },
  watch: {
    "signing.identityId"() {
      this.runCheck();
    },
    "signing.allowMissingEntitlements"() {
      this.runCheck();
    },
    "form.remove_extensions"() {
      this.runCheck();
    },
    "form.custom_identifier"() {
      this.runCheck();
    },
    "device.udid"() {
      this.runCheck();
    },
  },
  created() {
    this.id = this.$route.params.id;
    this.uploadSeq = 0;
    this.checkSeq = 0;
    this.reportStream = createSigningReportStream();

    this.fetchData();
  },
  mounted() {
    this.initWebSocket();
  },
  unmounted() {
    this.closeWebSocket();
    this.stopUpdateLog();
    // A running installation owns the uploaded IPA; otherwise drop it.
    if (!this.loading) {
      this.discardUploadedIpa();
    }
  },
  methods: {
    fetchData() {
      let _this = this;
      api.getDevice(_this.id).then((res) => {
        _this.device = res.data;
      });
      const accounts = api.getAccounts().then((res) => {
        const m = res.data || {};
        _this.accounts = Object.keys(m).map((k) => m[k]);
      }).catch(() => {
        _this.accounts = [];
      });
      const identities = api.getSigningIdentities().then((res) => {
        _this.signing.identities = res.data || [];
        if (_this.signing.identities.length === 1 && !_this.signing.identityId) {
          _this.signing.identityId = _this.signing.identities[0].id;
        }
      }).catch(() => {
        _this.signing.identities = [];
      });
      api.getAppList().then((res) => {
        _this.installedApps = res.data || [];
      }).catch(() => {
        _this.installedApps = [];
      });
      // Without any Apple account, default to the imported identities.
      Promise.all([accounts, identities]).then(() => {
        if (!_this.signing.modeChosen && _this.accounts.length === 0 &&
          _this.signing.identities.length > 0) {
          _this.applySigningMode(externalMode);
        }
      });
    },
    setSigningMode(mode) {
      this.signing.modeChosen = true;
      this.applySigningMode(mode);
    },
    applySigningMode(mode) {
      if (this.loading || this.signing.mode === mode) {
        return;
      }
      this.signing.mode = mode;
      this.signing.check = null;
      this.signing.checkError = null;
      this.recommendedAccount = "";
      this.recommendedIdentityId = 0;
      if (mode === externalMode) {
        // External signing only reads IPAs uploaded to this server.
        this.setInstallMode("file");
        this.prepareExternalIpa();
      } else {
        this.uploadSeq++;
        this.signing.uploading = false;
        this.discardUploadedIpa();
      }
    },
    manageIdentities() {
      this.$router.push({ name: "account", query: { section: "signing-identities" } });
    },
    formatDate(value) {
      const date = dayjs(value);
      return value && date.isValid() && date.year() > 1 ? date.format("YYYY-MM-DD") : "—";
    },
    codeText(code, message) {
      return signingCodeText(code, message, (key) => this.$t(key));
    },
    classText(signingClass) {
      const key = `signing.classes.${signingClass}`;
      const text = this.$t(key);
      return text !== key ? text : signingClass;
    },
    // prepareExternalIpa uploads the selected IPA, then checks it.
    async prepareExternalIpa() {
      if (!this.isExternal || this.installMode !== "file" || this.files.length === 0) {
        return;
      }
      const file = this.files[0];
      const seq = ++this.uploadSeq;
      this.discardUploadedIpa();
      this.signing.checkError = null;
      this.signing.uploading = true;
      try {
        const formData = new FormData();
        formData.append("files", file);
        const data = await api.upload(formData);
        if (seq !== this.uploadSeq) {
          // Superseded by another file or by a mode change.
          api.clean(data[0]).catch(() => {});
          return;
        }
        this.signing.uploaded = data[0];
      } catch (err) {
        if (seq === this.uploadSeq) {
          this.signing.checkError = requestErrorOf(err);
        }
        return;
      } finally {
        if (seq === this.uploadSeq) {
          this.signing.uploading = false;
        }
      }
      await this.runCheck();
    },
    discardUploadedIpa() {
      const uploaded = this.signing.uploaded;
      this.signing.uploaded = null;
      this.signing.check = null;
      if (uploaded) {
        api.clean(uploaded).catch(() => {});
      }
    },
    rerunCheck() {
      if (this.signing.uploaded) {
        this.runCheck();
      } else {
        this.prepareExternalIpa();
      }
    },
    // runCheck asks the server for the signing plan of the current choices.
    // The previous plan stays in place while the new one is computed so the
    // entitlement waiver does not flicker; install stays disabled meanwhile.
    async runCheck() {
      if (!this.isExternal || this.loading) {
        return;
      }
      const seq = ++this.checkSeq;
      const signing = this.signing;
      signing.checkError = null;
      if (!signing.uploaded || !signing.identityId || !this.device.udid) {
        signing.check = null;
        signing.checking = false;
        return;
      }

      signing.checking = true;
      try {
        const res = await api.checkSigningIdentity(signing.identityId, {
          ipa_path: signing.uploaded.path,
          udid: this.device.udid,
          remove_extensions: this.form.remove_extensions,
          allow_missing_entitlements: signing.allowMissingEntitlements,
          custom_identifier: this.form.custom_identifier,
        });
        if (seq === this.checkSeq) {
          signing.check = res.data || null;
        }
      } catch (err) {
        if (seq === this.checkSeq) {
          signing.check = null;
          signing.checkError = requestErrorOf(err);
        }
      } finally {
        if (seq === this.checkSeq) {
          signing.checking = false;
        }
      }
    },
    resetSigningReport() {
      this.reportStream = createSigningReportStream();
      this.report.plan = null;
      this.report.failure = null;
      this.report.verified = false;
    },
    applySigningReport(report) {
      switch (report.stage) {
        case "plan":
          this.report.plan = summarizePlan(report, report.blocking);
          break;
        case "failure":
          this.report.failure = report;
          break;
        case "verified":
          this.report.verified = true;
          break;
      }
    },
    appendStreamOutput(output) {
      output.reports.forEach(this.applySigningReport);
      this.log.newcontent += output.text;
    },
    // onInstallFinished runs once the stream reported the final result.
    onInstallFinished() {
      this.appendStreamOutput(this.reportStream.flush());
      this.loading = false;
      if (this.submittedMode === externalMode) {
        // The server consumes the uploaded IPA; a request rejected before
        // signing leaves it behind, so release it in every case.
        this.discardUploadedIpa();
      }
    },
    async onSubmit(e) {
      let _this = this;

      if (!_this.validateForm("#form")) {
        return;
      }
      if (!_this.canInstall) {
        return;
      }
      const external = _this.isExternal;
      if (_this.installMode === "source" && !_this.sourceSelection) {
        toast.error(this.$t("install.form.source.select_build"));
        return;
      }

      _this.loading = true;
      _this.submittedMode = _this.signing.mode;
      _this.resetSigningReport();
      _this.log.output = "";
      _this.log.newcontent = "";
      _this.log.show = true;

      _this.stopUpdateLog();
      _this.startUpdateLog();
      _this.log.output += "checking device status...\n";
      try {
        _this.log.output += `connection mode: ${_this.device.connection}\n`;
        if (_this.device.connection === "Lockdown") {
          _this.log.output += `product type: ${_this.device.product_type}\n`;
          _this.log.output += `product version: ${_this.device.product_version}\n`;
          _this.log.output += `developer mode: ${_this.device.developer_mode_status ? "enabled" : "disabled"}\n`;
          _this.log.output += `personalized image: ${_this.device.personalized_image_mounted ? "mounted" : "not mounted"}\n`;

          await _this.checkAfcService(_this.id);
        }

        let ipa;
        let source;
        if (external) {
          ipa = _this.signing.uploaded;
          _this.log.output += `IPA: ${ipa.name}\n`;
        } else if (_this.installMode === "file") {
          let formData = new FormData();
          for (let i = 0; i < _this.files.length; i++) {
            let file = _this.files[i];
            formData.append("files", file);
          }
          _this.log.output += "IPA uploading...\n";
          let data = await api.upload(formData)
          ipa = data[0];
        } else if (_this.installMode === "link") {
          _this.log.output += "IPA URL: " + _this.ipaUrl + "\n";
          ipa = {
            name: _this.ipaUrl.split('/').pop() || 'remote.ipa',
            path: _this.ipaUrl,
            icon: '',
            bundle_identifier: '',
            version: '',
          };
        } else {
          const selection = _this.sourceSelection;
          const build = selection.build;
          _this.log.output += `Source: ${selection.url} ${build.version} ${build.name}\n`;
          // The server resolves the download URL from the source; path and name are for display.
          ipa = {
            name: build.name,
            path: build.download_url,
            icon: '',
            bundle_identifier: build.bundle_id,
            version: build.version,
          };
          source = {
            kind: selection.kind,
            url: selection.url,
            filter: selection.filter,
            prerelease: selection.prerelease,
            auto_update: _this.form.auto_update,
            build_id: build.id,
          };
        }
        _this.ipa = ipa;
        // send start install msg
        _this.websocketsend(1, {
            ID: 0,
            ipa_name: _this.ipa.name,
            ipa_path: _this.ipa.path,
            device: _this.device.mac_addr,
            device_class: _this.device.device_class,
            udid: _this.device.udid,
            account: external ? "" : _this.form.account,
            password: external ? "" : _this.form.password,
            icon: _this.ipa.icon,
            bundle_identifier: _this.ipa.bundle_identifier,
            version: _this.ipa.version,
            custom_name: _this.form.custom_name.trim(),
            remove_extensions: _this.form.remove_extensions,
            custom_identifier: external ? _this.form.custom_identifier : "",
            signing_mode: _this.signing.mode,
            signing_identity_id: external ? Number(_this.signing.identityId) : 0,
            allow_missing_entitlements: external && _this.signing.allowMissingEntitlements,
            source,
        });
      } catch (error) {
        console.log(error);
        _this.log.newcontent += error;
        _this.loading = false;
        toast.error(this.$t("install.toast.install_failed"));
        return;
      }
    },
    reset() {
      document.getElementById("form").reset();
    },
    goBack() {
      this.$router.push("/");
    },
    setInstallMode(mode) {
      // The inputs of the current mode keep what they show: keep their state too.
      if (this.installMode === mode) return;
      this.installMode = mode;
      this.files = [];
      this.ipaUrl = "";
      this.sourceInitial = null;
      this.sourceSelection = null;
    },
    onIpaUrlInput(e) {
      if (e.inputType === "insertFromPaste") {
        this.switchToSourceIfRepo();
      }
    },
    // A GitHub repository is not an IPA link: open it as a tracked source instead.
    switchToSourceIfRepo() {
      const url = this.ipaUrl.trim();
      if (this.installMode !== "link" || guessSourceKind(url) !== "github") {
        return;
      }
      this.setInstallMode("source");
      this.sourceInitial = { kind: "github", url };
    },
    onSourceSelection(selection) {
      this.sourceSelection = selection;
      this.recommendedAccount = "";
      if (!selection) {
        return;
      }

      // Reuse the account of the app already installed from this source on
      // this device. Source installs sign with an Apple ID: external
      // certificate apps have no account to reuse.
      const url = selection.url.toLowerCase();
      const bundleId = selection.build.bundle_id;
      const appleIDApps = this.installedApps.filter((a) => a.signing_mode !== externalMode);
      const app =
        appleIDApps.find((a) => a.udid === this.device.udid && a.source.url.toLowerCase() === url) ||
        (bundleId && appleIDApps.find((a) => a.bundle_identifier === bundleId));
      if (app) {
        this.recommendedAccount = app.account;
        this.form.account = app.account;
        if (app.custom_name) {
          this.form.custom_name = app.custom_name;
        }
      }
    },
    async onFileChange(e) {
      this.files = e.target.files;
      this.recommendedAccount = "";
      this.recommendedIdentityId = 0;
      this.discardUploadedIpa();
      if (this.files.length > 0) {
        const file = this.files[0];
        try {
          const arrayBuffer = await file.arrayBuffer();
          const zip = await JSZip.loadAsync(arrayBuffer);

          const plistEntries = Object.keys(zip.files).filter((name) =>
            name.match(/^Payload\/[^/]+\.app\/Info\.plist$/)
          );

          for (const entry of plistEntries) {
            const plistData = await zip.files[entry].async("arraybuffer");
            const bundleId = parseBundleIdFromPlist(plistData);

            if (bundleId) {
              for (let app of this.installedApps) {
                if (app.bundle_identifier !== bundleId) {
                  continue;
                }
                if (this.isExternal) {
                  if (this.recommendIdentity(app)) {
                    break;
                  }
                  continue;
                }
                if (app.signing_mode === externalMode) {
                  continue;
                }
                this.recommendedAccount = app.account;
                this.form.account = app.account;
                if (app.custom_name) {
                  this.form.custom_name = app.custom_name;
                }
                break;
              }
              if (this.recommendedAccount || this.recommendedIdentityId) break;
            }
          }
        } catch (err) {
          console.error("Failed to read IPA bundle identifier:", err);
        }
      }
      this.prepareExternalIpa();
    },
    // recommendIdentity preselects the identity that signed an earlier
    // installation of the same app.
    recommendIdentity(app) {
      if (app.signing_mode !== externalMode) {
        return false;
      }
      const identity = this.signing.identities.find((item) => item.id === app.signing_identity_id);
      if (!identity) {
        return false;
      }
      this.recommendedIdentityId = identity.id;
      this.signing.identityId = identity.id;
      if (app.custom_name) {
        this.form.custom_name = app.custom_name;
      }
      if (app.custom_identifier) {
        this.form.custom_identifier = app.custom_identifier;
      }
      return true;
    },
    validateForm(id) {
      let form = document.querySelector(id);
      if (!form) {
        throw new Error(`not found form: ${id}`);
      }
      if (!form.checkValidity()) {
        form.reportValidity();
        return false;
      }
      return true;
    },
    showLoginDialog() {
        this.$refs.loginModal.show();
    },
    initWebSocket() {
      //初始化weosocket
      const wsuri =
        (location.protocol === "https:" ? "wss://" : "ws://") +
        location.host +
        "/ws/install"; //ws地址
      console.log(wsuri);
      this.websock = new WebSocket(wsuri);
      this.websock.onopen = this.websocketonopen;
      this.websock.onerror = this.websocketonerror;
      this.websock.onmessage = this.websocketonmessage;
      this.websock.onclose = this.websocketclose;
    },
    closeWebSocket() {
      this.websock.close();
    },

    websocketonopen() {
      console.log("WebSocket connect success.");
    },
    websocketonerror(e) {
      console.log("WebSocket connect failed.");
    },
    websocketonmessage(e) {
      let _this = this;
      // hide password string
      let line = e.data;

      if (line.indexOf("sealing regular file") !== -1) {
        return;
      }

      const output = _this.reportStream.push(line);
      _this.appendStreamOutput(output);

      // Installation successful.
      if (line.indexOf("Installation Succeeded") !== -1) {
        _this.onInstallFinished();
        toast.success(this.$t("install.toast.install_success"));
        return;
      }

      // Installation error
      if (line.indexOf("Installation Failed") !== -1) {
        _this.onInstallFinished();
        const failure = _this.report.failure;
        toast.error(failure
          ? _this.codeText(failure.code, failure.message)
          : _this.installFailureMessage());
        return;
      }
    },

    installFailureMessage() {
      return formatInstallFailureMessage(
        this.log.output + this.log.newcontent,
        (key) => this.$t(key),
      );
    },
    
    accountStatusLabel(status) {
      return formatAccountStatus(status, (key) => this.$t(key));
    },

    websocketsend(t, data) {
      let _this = this;
      if (typeof data !== 'string') {
        data = JSON.stringify(data);
      }
      const json = JSON.stringify({ t: t, d: data });
      console.log("--> ", json);
      if (_this.websock.readyState !== WebSocket.OPEN) {
        throw new Error("WebSocket is in CLOSING or CLOSED state.");
      }
      _this.websock.send(json);
    },

    websocketclose(e) {
      console.log(`connection closed (${e.code})`);
    },
    async checkAfcService(id) {
      let _this = this;
      try {
        await api.checkAfcService(id);
        _this.log.output += "afc service: OK!\n";
      } catch (error) {
        _this.log.output += `afc service: Failed!\n`;
        throw error;
      }
    },
    startUpdateLog() {
      let _this = this;
      _this.refreshLogInterval = setInterval(() => {
        if (_this.log.newcontent !== '') {
          _this.log.output += _this.log.newcontent;
          _this.log.newcontent = '';
          // The textbox follows the scroll to the bottom
          _this.$nextTick(() => {
            const textarea = document.querySelector("#log");
            textarea.scrollTop = textarea.scrollHeight;
          });
        }
      }, 500);
    },
    stopUpdateLog() {
      let _this = this;
      if (_this.refreshLogInterval) {
        clearInterval(_this.refreshLogInterval);
        _this.refreshLogInterval = null;
      }
    },
    isIPhone(item) {
      if (item.device_class) {
        return item.device_class.toLowerCase() == "iphone" || item.device_class.toLowerCase() == "ipad";
      }
      return item.name && (item.name.toLowerCase().includes("iphone") || item.name.toLowerCase().includes("ipad"));
    },
    isAppleTV(item) {
      if (item.device_class) {
        return item.device_class.toLowerCase() === "appletv";
      }
      return item.name && item.name.toLowerCase().includes("appletv");
    },
    canShowScreenshotAction(item) {
      return !!item && this.isAppleTV(item) && item.connection === "RPPairing";
    },
    async openScreenshotDialog() {
      if (!this.device || !this.device.id) {
        toast.error(this.$t("install.screenshot.toast.device_not_ready"));
        return;
      }
      this.screenshot.visible = true;
      this.screenshot.image = "";
      if (await this.mountDeviceImageAsync(this.device.id)) {
        this.captureScreenshot();
      }
    },
    closeScreenshotDialog() {
      this.screenshot.visible = false;
    },
    async mountDeviceImageAsync(deviceId) {
      try {
        this.screenshot.loading = true;
        this.screenshot.status = this.$t("install.screenshot.status.capturing");
        await api.mountDeviceImageAsync(deviceId);
        return true;
      } catch (error) {
        this.screenshot.status = this.$t("install.screenshot.status.failed");
        toast.error(error.message || this.$t("install.screenshot.toast.mount_failed"));
        this.screenshot.loading = false;
        return false;
      }
    },
    captureScreenshot() {
      if (!this.device || !this.device.id) {
        toast.error(this.$t("install.screenshot.toast.device_not_ready"));
        return;
      }
      this.screenshot.loading = true;
      this.screenshot.status = this.$t("install.screenshot.status.capturing");
      api
        .takeDeviceScreenshot(this.device.id)
        .then((res) => {
          const payload = (res && res.data) || {};
          if (payload.type === "screenshot" && payload.data) {
            this.screenshot.image = "data:image/jpeg;base64," + payload.data;
            this.screenshot.status = this.$t("install.screenshot.status.success");
          } else {
            this.screenshot.status = this.$t("install.screenshot.status.failed");
            toast.error(this.$t("install.screenshot.toast.failed"));
          }
        })
        .catch((err) => {
          this.screenshot.status = this.$t("install.screenshot.status.failed");
          // request.js already shows a toast on error; we just keep a status line.
          console.error("screenshot failed", err);
        })
        .finally(() => {
          this.screenshot.loading = false;
        });
    },
    downloadScreenshot() {
      if (!this.screenshot.image) {
        return;
      }
      const now = new Date();
      const pad = (n) => String(n).padStart(2, "0");
      const stamp =
        now.getFullYear() +
        pad(now.getMonth() + 1) +
        pad(now.getDate()) +
        "-" +
        pad(now.getHours()) +
        pad(now.getMinutes()) +
        pad(now.getSeconds());
      const filename = `screenshot-${stamp}.jpg`;
      const a = document.createElement("a");
      a.href = this.screenshot.image;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
    },
  },
};
</script>

<script setup>
import { truncateIP } from "@/utils/utils";
import AppleTVIcon from "@/assets/icons/appletv.svg";
import IPhoneIcon from "@/assets/icons/iphone.svg";
import WarningIcon from "@/assets/icons/warning.svg";
import PersonIcon from "@/assets/icons/person.badge.plus.svg";
import HelpIcon from "@/assets/icons/help.svg";
import CameraIcon from "@/assets/icons/camera.svg";
import RefreshIcon from "@/assets/icons/refresh.svg";
import DownloadIcon from "@/assets/icons/download.svg";
import FolderOpenIcon from "@/assets/icons/folder-open.svg";
import LinkIcon from "@/assets/icons/link.svg";
import SettingsIcon from "@/assets/icons/settings.svg";
import GithubIcon from "@/assets/icons/github.svg";
</script>
  
  <style scoped>
.line {
  text-align: center;
}

/* Install log follows the daisyUI mockup-code terminal look:
   background and text come from the active theme (--n / --nc).
   Placed on the page wrapper (higher specificity than the themed
   .textarea override in app.css) so it wins over the light field style. */
.atv-install-page .atv-install-log {
  background-color: hsl(var(--n) / var(--tw-bg-opacity, 1));
  border-color: hsl(var(--n) / var(--tw-bg-opacity, 1));
  color: hsl(var(--nc) / var(--tw-text-opacity, 1));
}

.atv-install-page .atv-install-log:focus {
  outline: 2px solid transparent;
  border-color: hsl(var(--nc) / 0.45);
  box-shadow: none;
}

/* Keep daisyUI join groups fused: the theme layer sets input/select/btn
   heights and radii that would otherwise split the joined edges apart. */
.atv-install-page .join {
  align-items: stretch;
}

.atv-install-page .join > .join-item:is(.input, .select, .file-input, .btn) {
  height: auto;
  min-height: 3rem;
  border-radius: 0;
}

.atv-install-page .join > .join-item:is(.input, .select, .file-input, .btn):first-child {
  border-start-start-radius: var(--rounded-btn, 0.5rem);
  border-end-start-radius: var(--rounded-btn, 0.5rem);
}

.atv-install-page .join > .join-item.btn:last-child {
  border-start-end-radius: var(--rounded-btn, 0.5rem);
  border-end-end-radius: var(--rounded-btn, 0.5rem);
}

.atv-install-page .join > .join-item.btn:first-child {
  border-start-start-radius: var(--rounded-btn, 0.5rem);
  border-end-start-radius: var(--rounded-btn, 0.5rem);
}

.atv-install-signing-mode > .btn {
  min-width: 0;
  white-space: normal;
  line-height: 1.2;
}

/* Compatibility plan of the external certificate mode, below the form. */
.atv-install-plan,
.atv-install-report {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  border: 1px solid var(--atv-border);
  border-radius: 12px;
  background: var(--atv-surface-alt);
  min-width: 0;
}

.atv-install-report {
  background: var(--atv-surface);
}

.atv-install-plan h5,
.atv-install-report h5 {
  margin: 0;
  font-size: .95rem;
  font-weight: 650;
  color: var(--atv-ink);
}

.atv-install-plan-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 32px;
}

.atv-install-plan-progress,
.atv-install-plan-hint {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  font-size: .86rem;
  color: var(--atv-muted);
}

.atv-install-plan-error {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px;
  border: 1px solid var(--atv-danger);
  border-radius: 10px;
  background: var(--atv-danger-soft);
  color: var(--atv-ink);
  font-size: .86rem;
  overflow-wrap: anywhere;
}

.atv-install-report summary {
  cursor: pointer;
  font-size: .86rem;
  color: var(--atv-muted);
}

/* Screenshot action sits on the device icon's bottom-right corner,
   like a profile-avatar edit badge. */
.atv-install-device-icon {
  position: relative;
}

.atv-install-device-icon .atv-install-screenshot-btn {
  position: absolute;
  right: -6px;
  bottom: -6px;
  width: 36px;
  min-width: 36px;
  height: 36px;
  min-height: 36px;
  padding: 0;
  border-radius: 9999px;
  box-shadow: var(--atv-menu-shadow);
}
</style>
