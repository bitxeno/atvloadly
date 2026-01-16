export function maskEmail(email) {
  if (!email || typeof email !== 'string') return email;
  const atIndex = email.indexOf('@');
  if (atIndex === -1) return email;
  const local = email.slice(0, atIndex);
  const domain = email.slice(atIndex);
  const maskedLocal = local.length <= 3 ? '***' : '***' + local.slice(3);
  return maskedLocal + domain;
}

export default { maskEmail };
