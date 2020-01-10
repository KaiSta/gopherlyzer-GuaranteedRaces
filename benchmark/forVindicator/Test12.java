package test;

public class Test12 extends Thread {
  static int x;
  static int y;
  static int z;
  static final Object m = new Object();
  static final Object n = new Object();
  static final Object o = new Object();
  static final Object p = new Object();
  static final Object q = new Object();

  static void sleepSec(float sec) {
    try {
	Thread.sleep((long)(sec*1000));
    } catch (Exception e) {
       throw new RuntimeException(e);
    }
  }


  @Override
  public void run() {
   	//T1
	synchronized(m) {		
		System.out.println("l(m)");
		x = 1;
		System.out.println("w(x)");
		System.out.println("u(m)");
	}
  } 

   public static class T2 extends Thread implements Runnable {
		public void run() {
			sleepSec(5);
			synchronized(m) {
				System.out.println("l(m)");
				x = 2;
				System.out.println("w(x)");
				System.out.println("u(m)");
			}			
		}
   }
   
   public static class T3 extends Thread implements Runnable {
		public void run() {
			try {
			sleepSec(10);
			int tmp = x;
			System.out.println("r(x)");
			final T4 thread4 = new T4();
			System.out.println("SIG(1)");
			thread4.start();
			thread4.join();
			System.out.println("WT(2)");
			tmp = x;
			System.out.println("r(x)");
			} catch (Exception e) {
				throw new RuntimeException();
			}
		}
   }
   
   public static class T4 extends Thread implements Runnable {
		public void run() {
			System.out.println("WT(1)");
			x = 3;
			System.out.println("w(x)");
			x = 4;
			System.out.println("w(x)");
		}
   }


   public static void main(String args[]) throws Exception {
	final Test12 thread1 = new Test12();
    final T2 thread2 = new T2();
	final T3 thread3 = new T3();
	

	thread1.start();
	thread2.start();
	thread3.start();
	

	thread1.join();
	thread2.join();
	thread3.join();

   }
}
