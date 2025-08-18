import type { CapacitorConfig } from '@capacitor/cli';

const config: CapacitorConfig = {
  appId: 'com.donetick.app',
  appName: 'Donetick',
  webDir: 'dist',
  
  plugins: {
    PushNotifications: {
      presentationOptions: ['badge', 'sound', 'alert'],
    },
    LocalNotifications: {
      smallIcon: "ic_stat_icon_config_sample",
      iconColor: "#488AFF",
      sound: "beep.wav",
    },
    GoogleAuth: {
      scopes: ['profile', 'email', 'openid'],
      clientId: process.env.VITE_APP_GOOGLE_CLIENT_ID,
      androidClientId: process.env.VITE_APP_ANDRIOD_CLIENT_ID,
      iosClientId: process.env.VITE_APP_IOS_CLIENT_ID,
  },
}
};

export default config;
