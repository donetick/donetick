import { LocalNotifications } from '@capacitor/local-notifications';
import { App as mobileApp } from '@capacitor/app';
import { PushNotifications } from '@capacitor/push-notifications'
import { PutNotificationTarget } from './utils/Fetcher';
import { Capacitor } from '@capacitor/core';
const localNotificationListenerRegistration = () => {
    LocalNotifications.addListener('localNotificationReceived', (notification) => {
      console.log('Notification received', notification);
    });
    LocalNotifications.addListener('localNotificationActionPerformed', (event) => {
      
      console.log('Notification action performed', event);
      if (event.actionId === 'tap') {
        console.log('Notification opened, navigate to chore', event.notification.extra.choreId); 
        window.location.href = `/chores/${event.notification.extra.choreId}`
      }
    });
  }
  const pushNotificationListenerRegistration = () => {
    PushNotifications.register();
    PushNotifications.addListener('registration', (token) => {
      if (Capacitor.isNativePlatform()) {
      const type = Capacitor.getPlatform() === 'android' ? 1 : 2; // 1 for android, 2 for ios
      PutNotificationTarget(type, token.value).then((response) => {
        console.log('Notification target updated', response);
      }
      ).catch((error) => {
        console.error('Error updating notification target', error);
      }
      );

      // TODO save the token in preferences and only send it if it has changed:
      console.log('Push registration success, token: ' + token.value);
      }
    }
    );
    PushNotifications.addListener('registrationError', (error) => {
      console.error('Error on registration: ' + JSON.stringify(error));
    }
    );
    PushNotifications.addListener('pushNotificationActionPerformed', fcmEvent => {
      
      if(fcmEvent.actionId === 'tap') {
        if (fcmEvent.notification.data.type === 'chore_due') {
          window.location.href = `/chores/${fcmEvent.notification.data.choreId}`
        }
        else {
          window.location.href = `/my/chores`
        }
    }
  }
  );
  }
 
  



  
  const registerCapacitorListeners = () => {
    if(!Capacitor.isNativePlatform()) {
      console.log('Not a native platform, skipping registration of native listeners');
      return 
    }
    localNotificationListenerRegistration();
    pushNotificationListenerRegistration();
    mobileApp.addListener('backButton', ({ canGoBack }) => {
        if (canGoBack) {
          window.history.back();
        } else {
          mobileApp.exitApp(); 
        }
      });
    

  }

    export { registerCapacitorListeners }