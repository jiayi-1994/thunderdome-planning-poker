export const pokerPointValues = ['0', '1/2', '1', '2', '3', '5', '8'];

export function availablePokerPoints(values: readonly string[]): string[] {
  return values.filter(value => pokerPointValues.includes(value));
}
