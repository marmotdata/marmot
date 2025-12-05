import type { PrismTheme } from 'prism-react-renderer';

// Light theme with darker earthy colors for better contrast
export const lightTheme: PrismTheme = {
  plain: {
    color: '#1f2937',
    backgroundColor: 'transparent',
  },
  styles: [
    {
      types: ['comment', 'prolog', 'doctype', 'cdata'],
      style: {
        color: '#4a674a',
        fontStyle: 'italic',
      },
    },
    {
      types: ['namespace'],
      style: {
        opacity: 0.7,
      },
    },
    {
      types: ['string', 'attr-value'],
      style: {
        color: '#35593b',
      },
    },
    {
      types: ['punctuation', 'operator'],
      style: {
        color: '#4a674a',
      },
    },
    {
      types: ['entity', 'url', 'symbol', 'number', 'boolean', 'variable', 'constant', 'property', 'regex', 'inserted'],
      style: {
        color: '#7b5935',
      },
    },
    {
      types: ['atrule', 'keyword', 'attr-name', 'selector'],
      style: {
        color: '#8d3718',
      },
    },
    {
      types: ['function', 'deleted', 'tag'],
      style: {
        color: '#b34822',
      },
    },
    {
      types: ['function-variable'],
      style: {
        color: '#b34822',
      },
    },
    {
      types: ['tag', 'selector', 'keyword'],
      style: {
        color: '#8d3718',
      },
    },
  ],
};

// Dark theme with brighter earthy tones for better readability
export const darkTheme: PrismTheme = {
  plain: {
    color: '#f3f4f6',
    backgroundColor: 'transparent',
  },
  styles: [
    {
      types: ['comment', 'prolog', 'doctype', 'cdata'],
      style: {
        color: '#a8c5a8',
        fontStyle: 'italic',
      },
    },
    {
      types: ['namespace'],
      style: {
        opacity: 0.7,
      },
    },
    {
      types: ['string', 'attr-value'],
      style: {
        color: '#b9d9b9',
      },
    },
    {
      types: ['punctuation', 'operator'],
      style: {
        color: '#d1e5d1',
      },
    },
    {
      types: ['entity', 'url', 'symbol', 'number', 'boolean', 'variable', 'constant', 'property', 'regex', 'inserted'],
      style: {
        color: '#f0d97e',
      },
    },
    {
      types: ['atrule', 'keyword', 'attr-name', 'selector'],
      style: {
        color: '#ffa77d',
      },
    },
    {
      types: ['function', 'deleted', 'tag'],
      style: {
        color: '#ffb899',
      },
    },
    {
      types: ['function-variable'],
      style: {
        color: '#ffb899',
      },
    },
    {
      types: ['tag', 'selector', 'keyword'],
      style: {
        color: '#ffa77d',
      },
    },
  ],
};
