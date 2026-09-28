<template>
  <div class="atv-signing-issues" v-if="groups.length > 0">
    <section
      v-for="group in groups"
      :key="group.severity"
      class="atv-signing-issue-group"
      :class="`atv-signing-issue-group--${group.severity}`"
    >
      <h5 class="atv-signing-issue-heading">
        {{ $t(`signing.severity.${group.severity}`) }}
      </h5>
      <ul>
        <li
          v-for="(issue, index) in visibleGroupIssues(group)"
          :key="`${issue.code}-${index}`"
          class="atv-signing-issue"
        >
          <span>{{ text(issue) }}</span>
          <span v-if="issue.message && issue.message !== text(issue)" class="atv-signing-issue-detail">
            {{ issue.message }}
          </span>
          <code v-if="issue.bundle" class="atv-signing-issue-bundle">{{ issue.bundle }}</code>
        </li>
      </ul>
      <div v-if="group.issues.length > maxVisible" class="mt-2">
        <button
          type="button"
          class="btn btn-xs btn-ghost atv-signing-issue-toggle"
          @click="toggleGroup(group.severity)"
        >
          {{ isExpanded(group.severity)
            ? $t("signing.issues.show_less")
            : $t("signing.issues.show_more", { count: group.issues.length - maxVisible })
          }}
        </button>
      </div>
    </section>
  </div>
</template>

<script>
import { groupIssuesBySeverity, issueText } from "@/utils/signing-report.mjs";

export default {
  name: "SigningIssueList",
  props: {
    issues: { type: Array, default: () => [] },
    maxVisible: { type: Number, default: 2 },
  },
  data() {
    return {
      expanded: {},
    };
  },
  computed: {
    filteredIssues() {
      return (this.issues || []).filter((issue) => issue && issue.code !== "revocation_not_checked");
    },
    groups() {
      return groupIssuesBySeverity(this.filteredIssues);
    },
  },
  methods: {
    text(issue) {
      return issueText(issue, (key) => this.$t(key));
    },
    isExpanded(severity) {
      return !!this.expanded[severity];
    },
    toggleGroup(severity) {
      this.expanded = {
        ...this.expanded,
        [severity]: !this.expanded[severity],
      };
    },
    visibleGroupIssues(group) {
      if (this.isExpanded(group.severity) || group.issues.length <= this.maxVisible) {
        return group.issues;
      }
      return group.issues.slice(0, this.maxVisible);
    },
  },
};
</script>

<style scoped>
.atv-signing-issues {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.atv-signing-issue-group {
  padding: 10px 12px;
  border: 1px solid var(--atv-border);
  border-left-width: 4px;
  border-radius: 12px;
  background: var(--atv-surface-alt);
}

.atv-signing-issue-group--error {
  border-color: var(--atv-danger);
  background: var(--atv-danger-soft);
}

.atv-signing-issue-group--warning {
  border-color: var(--atv-warning-border);
  background: var(--atv-warning-bg);
}

.atv-signing-issue-heading {
  margin: 0 0 6px;
  font-size: .78rem;
  font-weight: 760;
  letter-spacing: .02em;
  color: var(--atv-muted);
}

.atv-signing-issue-group--error .atv-signing-issue-heading {
  color: var(--atv-danger);
}

.atv-signing-issue-group--warning .atv-signing-issue-heading {
  color: var(--atv-warning-text);
}

.atv-signing-issue-group ul {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.atv-signing-issue {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: .86rem;
  line-height: 1.4;
  color: var(--atv-ink);
  overflow-wrap: anywhere;
}

.atv-signing-issue-detail,
.atv-signing-issue-bundle {
  font-size: .74rem;
  color: var(--atv-muted);
}

.atv-signing-issue-toggle {
  padding: 0 4px;
  height: 20px;
  min-height: 20px;
  font-size: .78rem;
  color: var(--atv-muted);
}

.atv-signing-issue-toggle:hover {
  background: transparent;
  text-decoration: underline;
}
</style>
