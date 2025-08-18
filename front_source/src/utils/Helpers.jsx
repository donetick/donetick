import moment from 'moment'
import { getAssetURL } from './TokenManager'

const isPlusAccount = userProfile => {
  return userProfile?.expiration && moment(userProfile?.expiration).isAfter()
}

const resolvePhotoURL = url => {
  if (!url) return ''
  if (url.startsWith('http') || url.startsWith('https')) {
    return url
  }
  if (url.startsWith('assets')) {
    return getAssetURL(url)
  }
  return url
}
export { isPlusAccount, resolvePhotoURL }
