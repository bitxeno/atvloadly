import CryptoJS from "crypto-js";

const key = CryptoJS.enc.Utf8.parse("GxWdhURAi+5yVeIg");
const iv = CryptoJS.enc.Utf8.parse("QF7CTvfyix113yNs");

function decrypt(str) {
  let encryptedHexStr = CryptoJS.enc.Hex.parse(str);
  let ciphertext = CryptoJS.enc.Base64.stringify(encryptedHexStr);
  let decrypt = CryptoJS.AES.decrypt(ciphertext, key, { iv: iv, mode: CryptoJS.mode.CBC, padding: CryptoJS.pad.Pkcs7 });
  let decryptedStr = decrypt.toString(CryptoJS.enc.Utf8);
  return decryptedStr.toString();
}

function encrypt(str) {
  let message = CryptoJS.enc.Utf8.parse(str);
  let encrypted = CryptoJS.AES.encrypt(message, key, { iv: iv, mode: CryptoJS.mode.CBC, padding: CryptoJS.pad.Pkcs7 });
  return encrypted.ciphertext.toString().toUpperCase();
}

export default {
  decrypt,
  encrypt,
};
