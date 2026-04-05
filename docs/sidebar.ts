import type { Sidebar } from 'vocs';

export const sidebar: Sidebar =
    [
        {
            text: 'Introduction',
            items: [
                {
                    text: 'What is OpenSigner?',
                    link: '/introduction/about',
                },
                {
                    text: 'Users',
                    link: '/introduction/users',
                },
                {
                    text: 'Setup',
                    link: '/introduction/setup',
                },
                {
                    text: 'Getting started',
                    link: '/introduction/getting-started',
                },
                {
                    text: 'Import shares from Openfort',
                    link: '/introduction/import-share-from-openfort',
                },
            ]
        },
        {
            text: 'Security',
            items: [
                {
                    text: 'Overview',
                    link: '/security/overview',
                },
                {
                    text: 'Recovery methods',
                    link: '/security/recovery-methods',
                },
                {
                    text: 'Deployment scenarios',
                    link: '/security/deployment-scenarios',
                },
                {
                    text: 'Threat analysis',
                    link: '/security/threat-analysis',
                },
                {
                    text: 'System integrity',
                    link: '/security/system-integrity',
                },
            ]
        },
        {
            text: 'Actions',
            items: [
                {
                    text: 'Create a key',
                    link: '/actions/signup',
                },
                {
                    text: 'Recover a key',
                    link: '/actions/login',
                },
                {
                    text: 'Use a key',
                    link: '/actions/operation',
                },
            ]
        },
        {
            text: 'Components',
            items: [
                {
                    text: 'Authentication',
                    link: '/components/auth',
                },
                {
                    text: 'iFrame',
                    link: '/components/iframe',
                },
                {
                    text: 'Hot storage',
                    link: '/components/hot_storage',
                },
                {
                    text: 'Cold storage',
                    items: [
                        {
                            text: 'Overview',
                            link: '/components/shield',
                        },
                        {
                            text: 'OTP for automatic recovery',
                            link: '/components/cold_storage/otp',
                        },
                    ],
                },
            ]
        },
        {
            text: "APIs",
            items: [
                {
                    text: 'Postman collection',
                    link: '/apis/postman',
                },
                {
                    text: 'Authentication service',
                    link: '/apis/auth_service',
                },
                {
                    text: 'Hot storage',
                    link: '/apis/hot_storage',
                },
                {
                    text: 'Cold storage',
                    link: '/apis/cold_storage',
                },
            ]
        },
    ];
