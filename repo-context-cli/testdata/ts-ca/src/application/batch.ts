export async function fanOut(a: Promise<void>, b: Promise<void>) {
  await Promise.all([a, b])
}
