import request from "@/utils/request";

export default {
  syncLang: (params) => {
    return request({
      url: '/api/lang/sync',
      method: "post",
      params
    });
  },
  getVersion: () => {
    return request({
      url: "/api/version",
      method: "get",
    });
  },
  getDevice: (id) => {
    return request({
      url: `/api/devices/${id}`,
      method: "get",
    });
  },
  getAccounts: (params) => {
    return request({
      url: "/api/accounts",
      method: "get",
      params,
    });
  },
  logoutAccount: (data) => {
    return request({
      url: "/api/accounts/logout",
      method: "post",
      data,
    });
  },
  getAccountDevices: (params) => {
    return request({
      url: "/api/accounts/devices",
      method: "get",
      timeout: 30000,
      params,
    });
  },
  deleteAccountDevice: (data) => {
    return request({
      url: "/api/accounts/devices/delete",
      method: "post",
      timeout: 30000,
      data,
    });
  },
  getCertificates: (params) => {
    return request({
      url: "/api/certificates",
      method: "get",
      timeout: 30000,
      params,
    });
  },
  revokeCertificate: (data) => {
    return request({
      url: "/api/certificates/revoke",
      method: "post",
      timeout: 30000,
      data,
    });
  },
  exportCertificate: (data) => {
    return request({
      url: "/api/certificates/export",
      method: "post",
      timeout: 60000,
      data,
      responseType: 'blob', // Important for file download
    });
  },
  importCertificate: (data) => {
    return request({
      url: "/api/certificates/import",
      method: "post",
      timeout: 60000,
      headers: {
        "Content-Type": "multipart/form-data",
      },
      data,
    });
  },
  // External signing identities (P12 + provisioning profile). Failures carry
  // err.result.data = {code, class, issues, app_count}; these calls never toast
  // on API errors so the caller can show the translated signing code.
  getSigningIdentities: () => {
    return request({
      url: "/api/signing/identities",
      method: "get",
      silent: true,
    });
  },
  importSigningIdentity: (data) => {
    return request({
      url: "/api/signing/identities/import",
      method: "post",
      timeout: 60000,
      silent: true,
      headers: {
        "Content-Type": "multipart/form-data",
      },
      data,
    });
  },
  replaceSigningIdentityProfile: (id, file) => {
    const data = new FormData();
    data.append("profile", file);
    return request({
      url: `/api/signing/identities/${id}/profile`,
      method: "post",
      timeout: 60000,
      silent: true,
      headers: {
        "Content-Type": "multipart/form-data",
      },
      data,
    });
  },
  deleteSigningIdentity: (id, force) => {
    return request({
      url: `/api/signing/identities/${id}/delete`,
      method: "post",
      timeout: 30000,
      silent: true,
      data: { force: !!force },
    });
  },
  checkSigningIdentity: (id, data) => {
    return request({
      url: `/api/signing/identities/${id}/check`,
      method: "post",
      timeout: 120000,
      silent: true,
      data,
    });
  },
  mountDeviceImageAsync: (id) => {
    return new Promise((resolve, reject) => {
      request({
        url: `/api/devices/${id}/mountimage`,
        timeout: 300000,
        method: "post",
      })
        .then((res) => {
          resolve(res.data);
        })
        .catch((err) => {
          reject(err);
        });
    });
  },
  takeDeviceScreenshot: (id) => {
    return request({
      url: `/api/devices/${id}/screenshot`,
      timeout: 300000,
      method: "post",
    });
  },
  checkAfcService: (id) => {
    return new Promise((resolve, reject) => {
      request({
        url: `/api/devices/${id}/check/afc`,
        timeout: 60000,
        method: "Post",
      })
        .then((res) => {
          resolve(res.data);
        })
        .catch((err) => {
          reject(err);
        });
    });
  },
  getDevices: (params) => {
    return request({
      url: "/api/devices",
      method: "get",
      params,
    });
  },
  scan: (params) => {
    return request({
      url: "/api/scan",
      method: "get",
      params,
    });
  },
  reload: (params) => {
    return request({
      url: "/api/reload",
      method: "get",
      params,
    });
  },
  pair: (data) => {
    return request({
      url: "/api/pair",
      method: "post",
      data,
    });
  },

  upload: (data) => {
    return new Promise((resolve, reject) => {
      request({
        url: "/api/upload",
        method: "post",
        timeout: 300000,
        headers: {
          "Content-Type": "multipart/form-data",
        },
        data,
      }).then((res) => {
        resolve(res.data);
      })
        .catch((err) => {
          reject(err);
        });
    });
  },

  getAppList: (params) => {
    return request({
      url: "/api/apps",
      method: "get",
      params,
    });
  },

  getInstallingApps: (params) => {
    return request({
      url: "/api/apps/installing",
      method: "get",
      params,
    });
  },

  deleteApp: (id) => {
    return request({
      url: `/api/apps/${id}/delete`,
      method: "post",
    });
  },

  refreshApp: (id) => {
    return request({
      url: `/api/apps/${id}/refresh`,
      method: "post",
    });
  },

  previewSource: (params) => {
    return request({
      url: "/api/sources/preview",
      method: "get",
      timeout: 60000,
      params,
    });
  },

  checkSourceUpdates: () => {
    return request({
      url: "/api/sources/check",
      method: "post",
      timeout: 120000,
    });
  },

  getSavedSources: () => {
    return request({
      url: "/api/sources/saved",
      method: "get",
    });
  },

  addSavedSource: (data) => {
    return request({
      url: "/api/sources/saved",
      method: "post",
      timeout: 60000,
      data,
    });
  },

  deleteSavedSource: (id) => {
    return request({
      url: `/api/sources/saved/${id}/delete`,
      method: "post",
    });
  },

  getSourceCatalog: (params) => {
    return request({
      url: "/api/sources/catalog",
      method: "get",
      timeout: 120000,
      params,
    });
  },

  downloadSourceBuild: (data) => {
    return request({
      url: "/api/sources/download",
      method: "post",
      timeout: 600000,
      silent: true,
      data,
    });
  },

  linkAppSource: (id, data) => {
    return request({
      url: `/api/apps/${id}/source`,
      method: "post",
      timeout: 60000,
      data,
    });
  },

  unlinkAppSource: (id) => {
    return request({
      url: `/api/apps/${id}/source/delete`,
      method: "post",
    });
  },

  updateAppFromSource: (id, data) => {
    return request({
      url: `/api/apps/${id}/source/update`,
      method: "post",
      timeout: 60000,
      data,
    });
  },

  clean: (data) => {
    return request({
      url: "/api/clean",
      method: "post",
      data,
    });
  },

  getSettings: (params) => {
    return request({
      url: "/api/settings",
      method: "get",
      params,
    });
  },

  saveNotificationSettings: function (data) {
    return request({
      url: "/api/settings/notification",
      method: "post",
      data,
    });
  },

  saveTaskSettings: function (data) {
    return request({
      url: "/api/settings/task",
      method: "post",
      data,
    });
  },
  
  saveNetworkSettings: function (data) {
    return request({
      url: "/api/settings/network",
      method: "post",
      data,
    });
  },

  saveUpdateSettings: function (data) {
    return request({
      url: "/api/settings/update",
      method: "post",
      data,
    });
  },


  getServiceStatus: () => {
    return request({
      url: `/api/service/status`,
      method: "get",
    });
  },

  sendNotify: (params) => {
    return request({
      url: `/api/notify/send`,
      method: "get",
      params,
    });
  },
  sendTestNotify: (data) => {
    return request({
      url: `/api/notify/send/test`,
      method: "post",
      timeout: 30000,
      data,
    });
  },
  updateCoreADI: () => {
    return request({
      url: "/api/settings/update/coreadi",
      method: "post",
      timeout: 600000,
    });
  },
  importPair: (data) => {
    return request({
      url: "/api/pair/import",
      method: "post",
      timeout: 30000,
      headers: {
        "Content-Type": "multipart/form-data",
      },
      data,
    });
  },
  scanWireless: () => {
    return request({
      url: "/api/scan/wireless",
      method: "get",
      timeout: 30000,
    });
  },
};
