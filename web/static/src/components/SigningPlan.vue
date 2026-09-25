<template>
  <div class="atv-signing-plan">
    <div class="atv-signing-plan-status">
      <span
        class="atv-status"
        :class="summary.blocking ? 'atv-status--invalid' : 'atv-status--valid'"
      >
        {{ summary.blocking ? $t("signing.plan.blocking") : $t("signing.plan.ready") }}
      </span>
    </div>

    <dl class="atv-signing-plan-facts">
      <div>
        <dt>{{ $t("signing.plan.bundle_id") }}</dt>
        <dd v-if="summary.bundleIdRewritten" class="atv-signing-plan-rewrite">
          <code>{{ summary.mainBundleId }}</code>
          <span aria-hidden="true">→</span>
          <code>{{ summary.signedMainBundleId }}</code>
        </dd>
        <dd v-else>
          <code>{{ summary.mainBundleId || "—" }}</code>
          <span class="atv-signing-plan-muted">({{ $t("signing.plan.unchanged") }})</span>
        </dd>
      </div>
      <div v-if="summary.mainApplicationIdentifier">
        <dt>{{ $t("signing.plan.application_identifier") }}</dt>
        <dd><code>{{ summary.mainApplicationIdentifier }}</code></dd>
      </div>
      <div>
        <dt>{{ $t("signing.plan.removed_extensions") }}</dt>
        <dd v-if="summary.removedBundles.length > 0">
          <ul>
            <li v-for="path in summary.removedBundles" :key="path"><code>{{ path }}</code></li>
          </ul>
        </dd>
        <dd v-else class="atv-signing-plan-muted">{{ $t("signing.plan.none") }}</dd>
      </div>
      <div>
        <dt>{{ $t("signing.plan.device") }}</dt>
        <dd>
          <span class="atv-status" :class="deviceStatus.className">{{ deviceStatus.label }}</span>
        </dd>
      </div>
    </dl>

    <SigningIssueList :issues="summary.issues" />
  </div>
</template>

<script>
import SigningIssueList from "@/components/SigningIssueList.vue";
import { deviceCompatibilityIssues, normalizeSeverity } from "@/utils/signing-report.mjs";

export default {
  name: "SigningPlan",
  components: { SigningIssueList },
  props: {
    // Output of summarizePlan().
    summary: { type: Object, required: true },
  },
  computed: {
    deviceStatus() {
      const issues = deviceCompatibilityIssues(this.summary.issues);
      if (issues.some((issue) => normalizeSeverity(issue.severity) === "error")) {
        return { className: "atv-status--invalid", label: this.$t("signing.plan.device_problem") };
      }
      if (issues.length > 0) {
        return { className: "atv-status--warning", label: this.$t("signing.plan.device_warning") };
      }
      return { className: "atv-status--valid", label: this.$t("signing.plan.device_ok") };
    },
  },
};
</script>

<style scoped>
.atv-signing-plan {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.atv-signing-plan-status .atv-status {
  white-space: normal;
}

.atv-signing-plan-facts {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
}

.atv-signing-plan-facts > div {
  display: grid;
  grid-template-columns: minmax(120px, 34%) minmax(0, 1fr);
  gap: 10px;
  align-items: baseline;
  font-size: .86rem;
}

.atv-signing-plan-facts dt {
  color: var(--atv-muted);
  font-weight: 650;
  min-width: 0;
  overflow-wrap: anywhere;
}

.atv-signing-plan-facts dd {
  margin: 0;
  min-width: 0;
  overflow-wrap: anywhere;
}

.atv-signing-plan-facts ul {
  margin: 0;
  padding: 0;
  list-style: none;
}

.atv-signing-plan-rewrite {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 6px;
  align-items: baseline;
}

.atv-signing-plan-muted {
  color: var(--atv-muted);
  margin-left: 4px;
}

@media (max-width: 767px) {
  .atv-signing-plan-facts > div {
    grid-template-columns: minmax(0, 1fr);
    gap: 2px;
  }
}
</style>
