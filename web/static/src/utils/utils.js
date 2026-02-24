export function maskEmail(email) {
  if (!email || typeof email !== 'string') return email;
  const atIndex = email.indexOf('@');
  if (atIndex === -1) return email;
  const local = email.slice(0, atIndex);
  const domain = email.slice(atIndex);
  const maskedLocal = local.length <= 3 ? '***' : '***' + local.slice(3);
  return maskedLocal + domain;
}

export function getStringSimilarity(s1, s2) {
  if (!s1 || !s2) return 0;
  s1 = s1.toLowerCase();
  s2 = s2.toLowerCase();
  if (s1 === s2) return 1;

  const longer = s1.length > s2.length ? s1 : s2;
  const shorter = s1.length > s2.length ? s2 : s1;

  if (longer.length === 0) return 1.0;

  const editDistance = (a, b) => {
    const matrix = [];
    for (let i = 0; i <= a.length; i++) {
      matrix[i] = [i];
    }
    for (let j = 0; j <= b.length; j++) {
      matrix[0][j] = j;
    }

    for (let i = 1; i <= a.length; i++) {
      for (let j = 1; j <= b.length; j++) {
        if (a[i - 1] === b[j - 1]) {
          matrix[i][j] = matrix[i - 1][j - 1];
        } else {
          matrix[i][j] = Math.min(
            matrix[i - 1][j - 1] + 1,
            matrix[i][j - 1] + 1,
            matrix[i - 1][j] + 1
          );
        }
      }
    }
    return matrix[a.length][b.length];
  };

  return (longer.length - editDistance(longer, shorter)) / parseFloat(longer.length);
}

export default { maskEmail, getStringSimilarity };
