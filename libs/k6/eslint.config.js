import js from "@eslint/js";
import prettier from "eslint-plugin-prettier/recommended";
import globals from "globals";

export default [
    {
        ignores: [".cache/", "public/", "node_modules/", "build/"],
    },
    js.configs.recommended,
    prettier,
    {
        languageOptions: {
            ecmaVersion: 13,
            sourceType: "module",
            globals: {
                ...globals.browser,
                ...globals.node,
                __ENV: "readonly",
            },
        },
        rules: {
            "prettier/prettier": "error",
            "arrow-body-style": "warn",
            "camelcase": "off",
            "object-curly-newline": "off",
            "operator-linebreak": "off",
            "no-shadow": "off",
            "max-len": ["error", 120],
            "no-underscore-dangle": "off",
            "no-unused-vars": "off",
        },
    },
];
